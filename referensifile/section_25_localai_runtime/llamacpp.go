package localai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/teetah2402/flowork/internal/fsutil"
)

// BUG-rc120 / Gemini review ACC: SHA256 integrity verify sebelum exec
// llama-server binary. Tanpa ini, attacker yang bisa write ke
// ~/.flowork/bin/ = RCE (binary spawned as host OS process). Gemini
// quote: "Exec binary buatan luar adalah target RCE paling legit.
// WAJIB terapkan verifikasi integritas SHA256 (Checksum) terhadap
// binary tersebut sebelum proses exec.Command dijalankan."
//
// Integrity model:
//  1. Flowork release bundle llama-server + SHA256SUMS file di
//     ~/.flowork/bin/SHA256SUMS (one line per binary: <hex> <name>)
//  2. Before exec.Command, resolve binary path + read SHA256SUMS +
//     compute actual hash of binary file.
//  3. Mismatch = ErrBinaryIntegrity, kernel-panic untuk localai daemon
//     (bukan start subprocess).
//  4. Developer mode: FLOWORK_LOCALAI_SKIP_INTEGRITY=1 env bypass
//     (warning logged). Untuk prod WAJIB unset.

// ErrBinaryIntegrity means the llama-server binary hash doesn't match
// the manifest. Refuse to exec — potential supply-chain compromise.
var ErrBinaryIntegrity = errors.New("localai: llama-server binary integrity check failed — refusing to exec")

// llamaCppRuntime — Phase A: manage llama-server subprocess (llama.cpp HTTP
// server mode), komunikasi via localhost JSON-RPC.
//
// Lifecycle:
//  1. Spawn subprocess di Open() dengan flag `--model` + `--port` + `--ctx-size`
//  2. Poll /health sampai ready atau StartupTimeout
//  3. Complete/Stream post ke /completion endpoint
//  4. Close() kirim SIGTERM, wait graceful, force kill kalau stuck >5s
//
// Binary location: `~/.flowork/bin/llama-server[.exe]` — Flowork release
// bundle per-OS (download via `flowork bin pull llama-server`). Kalau tidak
// ketemu, fallback cari di PATH.
type llamaCppRuntime struct {
	cfg       Config
	cmd       *exec.Cmd
	baseURL   string
	client    *http.Client
	startedAt time.Time

	mu      sync.RWMutex
	closed  atomic.Bool
	lastErr string
}

func newLlamaCppRuntime(cfg Config) (*llamaCppRuntime, error) {
	binPath, err := resolveLlamaServerBin()
	if err != nil {
		return nil, err
	}

	// Gemini rc120 ACC: SHA256 integrity verify before exec (anti-RCE).
	if os.Getenv("FLOWORK_LOCALAI_SKIP_INTEGRITY") != "1" {
		if err := verifyBinaryIntegrity(binPath); err != nil {
			return nil, err
		}
	} else {
		fmt.Fprintln(os.Stderr, "localai: WARN FLOWORK_LOCALAI_SKIP_INTEGRITY=1 — SHA256 verify bypassed. Developer mode. Unset di prod.")
	}

	modelPath, err := resolveModelPath(cfg.Model)
	if err != nil {
		return nil, err
	}

	port := cfg.Port
	if port == 0 {
		port, err = findEphemeralPort()
		if err != nil {
			return nil, fmt.Errorf("localai llamacpp: find port: %w", err)
		}
	}

	threads := cfg.Threads
	if threads == 0 {
		threads = runtime.NumCPU() / 2
		if threads < 1 {
			threads = 1
		}
	}

	args := []string{
		"--model", modelPath,
		"--port", fmt.Sprintf("%d", port),
		"--ctx-size", fmt.Sprintf("%d", cfg.ContextSize),
		"--threads", fmt.Sprintf("%d", threads),
		"--host", "127.0.0.1", // NEVER bind 0.0.0.0 — LAN exposure risk
	}
	if cfg.GPULayers > 0 {
		args = append(args, "--n-gpu-layers", fmt.Sprintf("%d", cfg.GPULayers))
	}

	// Spawn subprocess. stdout+stderr silenced by default to avoid noise;
	// caller dapat inspect via Status.LastErr kalau gagal.
	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("localai llamacpp: start: %w", err)
	}

	rt := &llamaCppRuntime{
		cfg:       cfg,
		cmd:       cmd,
		baseURL:   fmt.Sprintf("http://127.0.0.1:%d", port),
		client:    &http.Client{Timeout: 60 * time.Second},
		startedAt: time.Now(),
	}

	// Wait for health.
	deadline := time.Now().Add(cfg.StartupTimeout)
	for time.Now().Before(deadline) {
		if rt.ping(context.Background()) {
			return rt, nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Startup failed — cleanup.
	_ = rt.Close()
	return nil, fmt.Errorf("localai llamacpp: startup timeout after %s (model=%s)", cfg.StartupTimeout, cfg.Model)
}

func (r *llamaCppRuntime) ping(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", r.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req = req.WithContext(cctx)
	resp, err := r.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (r *llamaCppRuntime) Complete(ctx context.Context, req Request) (Response, error) {
	if r.closed.Load() {
		return Response{}, ErrNotReady
	}
	start := time.Now()

	payload := map[string]any{
		"prompt":      req.Prompt,
		"n_predict":   req.MaxTokens,
		"temperature": req.Temperature,
		"top_p":       req.TopP,
		"stop":        req.Stop,
		"stream":      false,
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.baseURL+"/completion", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("localai llamacpp: complete: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		Content         string `json:"content"`
		StopType        string `json:"stop_type"`
		TokensPredicted int    `json:"tokens_predicted"`
		TokensEvaluated int    `json:"tokens_evaluated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Response{}, fmt.Errorf("localai llamacpp: decode: %w", err)
	}

	return Response{
		Text:             raw.Content,
		FinishReason:     raw.StopType,
		PromptTokens:     raw.TokensEvaluated,
		CompletionTokens: raw.TokensPredicted,
		TotalMS:          time.Since(start).Milliseconds(),
	}, nil
}

func (r *llamaCppRuntime) Stream(ctx context.Context, req Request, onToken func(string) error) (Response, error) {
	if r.closed.Load() {
		return Response{}, ErrNotReady
	}
	start := time.Now()

	payload := map[string]any{
		"prompt":      req.Prompt,
		"n_predict":   req.MaxTokens,
		"temperature": req.Temperature,
		"top_p":       req.TopP,
		"stop":        req.Stop,
		"stream":      true,
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", r.baseURL+"/completion", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("localai llamacpp: stream: %w", err)
	}
	defer resp.Body.Close()

	var full bytes.Buffer
	var stopType string
	var promptTokens, completionTokens int

	dec := json.NewDecoder(resp.Body)
	for dec.More() {
		var chunk struct {
			Content         string `json:"content"`
			Stop            bool   `json:"stop"`
			StopType        string `json:"stop_type"`
			TokensPredicted int    `json:"tokens_predicted"`
			TokensEvaluated int    `json:"tokens_evaluated"`
		}
		if err := dec.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return Response{}, fmt.Errorf("localai llamacpp: stream decode: %w", err)
		}
		if chunk.Content != "" {
			full.WriteString(chunk.Content)
			if onToken != nil {
				if err := onToken(chunk.Content); err != nil {
					return Response{Text: full.String(), FinishReason: "abort"}, err
				}
			}
		}
		if chunk.Stop {
			stopType = chunk.StopType
			promptTokens = chunk.TokensEvaluated
			completionTokens = chunk.TokensPredicted
			break
		}
	}

	return Response{
		Text:             full.String(),
		FinishReason:     stopType,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalMS:          time.Since(start).Milliseconds(),
	}, nil
}

func (r *llamaCppRuntime) Health(ctx context.Context) Status {
	st := Status{
		Backend:  BackendLlamaCpp,
		Model:    r.cfg.Model,
		UptimeMS: time.Since(r.startedAt).Milliseconds(),
	}
	if r.cmd != nil && r.cmd.Process != nil {
		st.SubprocPID = r.cmd.Process.Pid
	}
	st.Ready = !r.closed.Load() && r.ping(ctx)
	r.mu.RLock()
	st.LastErr = r.lastErr
	r.mu.RUnlock()
	return st
}

func (r *llamaCppRuntime) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}
	// Graceful SIGTERM (Windows: Process.Kill = TerminateProcess; fine).
	_ = r.cmd.Process.Signal(os.Interrupt)

	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		_ = r.cmd.Process.Kill()
		<-done
		return nil
	}
}

// resolveLlamaServerBin mencari llama-server binary. Priority (portable per
// RULE EMAS §1.4 — semua di project root, BUKAN user home):
//  1. env FLOWORK_INFERENCE_BIN (override)
//  2. <project_root>/bin/llamacpp/llama-server[.exe]  (Flowork bundled)
//  3. <project_root>/bin/llama-server[.exe]            (legacy fallback)
//  4. $PATH lookup                                     (user-installed)
//
// Cross-OS: Windows pakai `.exe`, Unix tanpa extension.
func resolveLlamaServerBin() (string, error) {
	if v := os.Getenv("FLOWORK_INFERENCE_BIN"); v != "" {
		if _, err := fsutil.SafeStat(v); err == nil {
			return v, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("localai: getwd: %w", err)
	}
	projectRoot := cwd
	if filepath.Base(cwd) == "floworkos-go" {
		projectRoot = filepath.Dir(cwd)
	}
	binName := "llama-server"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	// 1) <project_root>/bin/llamacpp/<bin>
	bundled := fsutil.SafeJoin(projectRoot, "bin", "llamacpp", binName)
	if _, err := fsutil.SafeStat(bundled); err == nil {
		return bundled, nil
	}
	// 2) <project_root>/bin/<bin> (legacy)
	flat := fsutil.SafeJoin(projectRoot, "bin", binName)
	if _, err := fsutil.SafeStat(flat); err == nil {
		return flat, nil
	}
	// 3) Fallback to PATH.
	if p, err := exec.LookPath(binName); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("localai: %s tidak ditemukan di %s atau PATH. Download llama.cpp prebuilt server.exe ke <project_root>/bin/llamacpp/", binName, bundled)
}

// resolveModelPath mencari GGUF file di <project_root>/models/.
//
// Portable per RULE EMAS §1.4: lookup di <project_root>/models/, BUKAN
// .flowork/ user home (model travel sama project saat clone/migrate).
//
// rc178: kalau primary `name` ga ada, fallback ke largest available gguf di
// <project_root>/models/ + log warning. Defensive against hardcoded model
// name yang belum di-download.
func resolveModelPath(name string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("localai: getwd: %w", err)
	}
	projectRoot := cwd
	if filepath.Base(cwd) == "floworkos-go" {
		projectRoot = filepath.Dir(cwd)
	}
	modelsDir := fsutil.SafeJoin(projectRoot, "models")
	p := fsutil.SafeJoin(modelsDir, name)
	if _, err := fsutil.SafeStat(p); err == nil {
		return p, nil
	}
	// Primary not found — try fallback: largest available gguf.
	if fallback := findLargestLocalGGUF(projectRoot); fallback != "" {
		fbPath := fsutil.SafeJoin(modelsDir, fallback)
		if _, err := fsutil.SafeStat(fbPath); err == nil {
			fmt.Fprintf(os.Stderr, "localai: model %q tidak ditemukan, FALLBACK ke %q (largest available di <project_root>/models/)\n", name, fallback)
			return fbPath, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrModelMissing, p)
}

// findLargestLocalGGUF scan <project_root>/models/*.gguf, return basename
// of largest file. Empty kalau ga ada gguf available.
//
// Heuristic: bigger gguf = bigger params = better fallback. Saat warga
// ga punya specific model, pakai yang paling capable available.
func findLargestLocalGGUF(projectRoot string) string {
	dir := filepath.Join(projectRoot, "models")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var bestName string
	var bestSize int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gguf") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() > bestSize {
			bestSize = info.Size()
			bestName = e.Name()
		}
	}
	return bestName
}

// findEphemeralPort asks OS for a free TCP port (avoid collisions).
func findEphemeralPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// verifyBinaryIntegrity computes SHA256 of binary dan compare terhadap
// entry di SHA256SUMS manifest. Format manifest (one per line):
//
//	<64-hex-hash>  <filename>
//
// Lokasi manifest: sama dir dengan binary (sibling file SHA256SUMS).
// Fallback: `<binPath>.sha256` dengan satu baris hex hash.
//
// Error semantics:
//   - ErrBinaryIntegrity kalau hash mismatch
//   - Error lain (tidak ada manifest, binary unreadable) = refuse exec juga
//     kecuali dev-mode env set. Strict by default — fail closed.
func verifyBinaryIntegrity(binPath string) error {
	// 1. Compute actual SHA256 of binary.
	f, err := fsutil.SafeOpen(binPath)
	if err != nil {
		return fmt.Errorf("%w: open binary: %v", ErrBinaryIntegrity, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("%w: read binary: %v", ErrBinaryIntegrity, err)
	}
	actualHex := hex.EncodeToString(h.Sum(nil))

	// 2. Resolve manifest.
	binDir := filepath.Dir(binPath)
	binName := filepath.Base(binPath)
	manifestPath := fsutil.SafeJoin(binDir, "SHA256SUMS")

	expectedHex, mfErr := lookupManifestHash(manifestPath, binName)
	if mfErr != nil {
		// Fallback: sibling <binPath>.sha256 single-line.
		siblingPath := binPath + ".sha256"
		if data, err := fsutil.SafeReadFile(siblingPath); err == nil {
			parts := strings.Split(string(data), " ")
			if len(parts) > 0 {
				expectedHex = strings.TrimSpace(parts[0])
			}
		} else {
			return fmt.Errorf("%w: manifest missing — expected %s atau %s. Untuk dev-mode set FLOWORK_LOCALAI_SKIP_INTEGRITY=1", ErrBinaryIntegrity, manifestPath, siblingPath)
		}
	}

	// 3. Compare constant-time (binary digest, not secret, tapi tetap defensif).
	if !strings.EqualFold(actualHex, expectedHex) {
		return fmt.Errorf("%w: hash mismatch — actual=%s expected=%s. Binary mungkin di-tampering; delete + re-download via `flowork bin pull llama-server`", ErrBinaryIntegrity, actualHex, expectedHex)
	}
	return nil
}

// lookupManifestHash parses SHA256SUMS line-per-line, cari entry yang
// filename-nya match binName.
func lookupManifestHash(manifestPath, binName string) (string, error) {
	data, err := fsutil.SafeReadFile(manifestPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Manifest format: "<hex>  <name>" (GNU coreutils sha256sum format,
		// can have leading '*' before name for binary mode).
		fname := strings.TrimPrefix(fields[len(fields)-1], "*")
		if fname == binName || filepath.Base(fname) == binName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("binName %q not found in %s", binName, manifestPath)
}
