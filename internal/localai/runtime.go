// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flow_router
// Locked at: 2026-05-30
// Reason: Section 25 phase 2 LocalAI runtime — llama-server subprocess
//   lifecycle (start/stop/health). Single-instance model loaded at a time.
//   Phase 3 (multi-model swap, GPU layer config, llama.cpp build self-
//   compile) → tambah file baru.
//
// runtime.go — Section 25 phase 2: llama-server subprocess.

package localai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Runtime — manages llama-server subprocess.
type Runtime struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	modelName string
	port      int
	binPath   string
	cli       *http.Client
}

// NewRuntime — caller supply path ke llama-server binary (or assume
// PATH-resolved "llama-server"). Default port 8088 (anti collide dengan
// kernel 1987 / router 2402 / mDNS 5353).
func NewRuntime(binPath string, port int) *Runtime {
	if binPath == "" {
		binPath = "llama-server"
	}
	if port <= 0 {
		port = 8088
	}
	return &Runtime{
		binPath: binPath,
		port:    port,
		cli:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Start — spawn llama-server with model file. Stop existing first kalau ada.
// Caller pass GGUF path. Best-effort: kalau binary tidak ada, return error.
func (r *Runtime) Start(modelName, ggufPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil && r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
		_, _ = r.cmd.Process.Wait()
		r.cmd = nil
	}
	if modelName == "" || ggufPath == "" {
		return fmt.Errorf("model_name + gguf_path required")
	}
	args := []string{
		"-m", ggufPath,
		"--port", strconv.Itoa(r.port),
		"--host", "127.0.0.1",
	}
	cmd := exec.Command(r.binPath, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start llama-server: %w", err)
	}
	r.cmd = cmd
	r.modelName = modelName
	// Wait for health up to 30s.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if r.healthy() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("llama-server start timeout (port %d)", r.port)
}

// Stop — terminate process. Best-effort.
func (r *Runtime) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}
	_ = r.cmd.Process.Kill()
	_, _ = r.cmd.Process.Wait()
	r.cmd = nil
	r.modelName = ""
	return nil
}

// Status — return current state.
type Status struct {
	Running   bool   `json:"running"`
	ModelName string `json:"model_name"`
	Port      int    `json:"port"`
	Healthy   bool   `json:"healthy"`
}

func (r *Runtime) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := Status{
		Port:      r.port,
		ModelName: r.modelName,
	}
	if r.cmd != nil && r.cmd.Process != nil {
		st.Running = true
		// Health check without holding lock-after-IO is fine — race accepted.
	}
	if st.Running {
		st.Healthy = r.healthyUnlocked()
	}
	return st
}

func (r *Runtime) healthy() bool {
	return r.healthyUnlocked()
}

func (r *Runtime) healthyUnlocked() bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", r.port)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := r.cli.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return strings.Contains(string(body), "ok") || resp.StatusCode == 200
}
