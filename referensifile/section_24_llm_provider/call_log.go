// call_log.go — rc188 append-only log setiap panggilan LLM provider
// untuk dashboard overview real data.
//
// Ayah directive 2026-04-20 post-rc187: widget dashboard yang ngak real
// harus dibikin real. Problem: openrouter.json cuma simpan usage_usd
// total cumulative, ngak per-request. Dashboard estimate token/per-day
// dari total bikin angka fake.
//
// Fix: setiap completeWithRateLimitRetry sukses, append 1 baris JSON
// dengan token count + timestamp + model. Dashboard baca file ini untuk
// Daily Breakdown, Usage Trend, KPI 7d/30d/avg yang sekarang real.
//
// File path:
//
//	FLOWORK_WORKSPACE/state/analytics/openrouter_calls.jsonl (prefer)
//	atau ~/.flowork/analytics/openrouter_calls.jsonl (fallback)
//
// Format per baris:
//
//	{"ts":"2026-04-20T17:00:00Z","model":"...","input":N,"output":N,
//	 "cached":N,"finish":"end_turn","agent":"watcher"}
package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type callLogEntry struct {
	TS     string `json:"ts"`
	Model  string `json:"model"`
	Input  int    `json:"input"`
	Output int    `json:"output"`
	Cached int    `json:"cached"`
	Finish string `json:"finish,omitempty"`
	Agent  string `json:"agent,omitempty"`
}

var callLogMu sync.Mutex

// LogOpenRouterCall appends a single entry. Panggil dari openai.go
// setelah sukses response (non-error path). Fire-and-forget: kegagalan
// log TIDAK boleh gagal-kan request asli — best-effort.
func LogOpenRouterCall(model string, input, output, cached int, finish string) {
	callLogMu.Lock()
	defer callLogMu.Unlock()

	path := callLogPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	entry := callLogEntry{
		TS:     time.Now().UTC().Format(time.RFC3339),
		Model:  model,
		Input:  input,
		Output: output,
		Cached: cached,
		Finish: finish,
		Agent:  strings.TrimSpace(os.Getenv("FLOWORK_AGENT_HANDLE")),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

// callLogPath resolve path file log. Prefer FLOWORK_WORKSPACE/state/...
// supaya dashboard di GUI (baca dari workspace) langsung dapat data.
// Fallback ~/.flowork/analytics/ kalau workspace env kosong.
func callLogPath() string {
	if ws := strings.TrimSpace(os.Getenv("FLOWORK_WORKSPACE")); ws != "" {
		return filepath.Join(ws, "state", "analytics", "openrouter_calls.jsonl")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".flowork", "analytics", "openrouter_calls.jsonl")
}
