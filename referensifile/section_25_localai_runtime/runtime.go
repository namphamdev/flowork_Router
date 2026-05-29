package localai

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Runtime abstracts local inference — opaque ke caller. Implementasi bisa:
//   - Phase A: llama.cpp subprocess (llamacpp.go)
//   - Phase B: pure Go inference (infer.go, Sprint 4+)
//
// Migration path: ubah Config.Backend tanpa ubah caller.
type Runtime interface {
	// Complete runs inference given a prompt and returns the full completion.
	Complete(ctx context.Context, req Request) (Response, error)

	// Stream runs inference and pushes tokens via callback until done or ctx
	// cancelled. Useful untuk Healer that want to abort saat <2s deadline hit.
	Stream(ctx context.Context, req Request, onToken func(string) error) (Response, error)

	// Health reports runtime status (ready/degraded/unavailable).
	Health(ctx context.Context) Status

	// Close releases runtime resources (kill subprocess, unload model).
	Close() error
}

// Backend selects the implementation tier.
type Backend string

const (
	// BackendLlamaCpp — Phase A: managed llama.cpp subprocess HTTP server.
	BackendLlamaCpp Backend = "llamacpp"

	// BackendPureGo — Phase B: native Go inference (Sprint 4+).
	BackendPureGo Backend = "purego"

	// BackendAuto — runtime pick: PureGo kalau model tersedia + platform support,
	// kalau tidak fallback ke LlamaCpp.
	BackendAuto Backend = "auto"
)

// Config holds runtime configuration.
type Config struct {
	// Backend selects implementation. Default: BackendAuto.
	Backend Backend

	// Model is the GGUF model filename (relative to ~/.flowork/models/).
	// Example: "deepseek-coder-v2-lite-q4.gguf", "qwen2.5-0.5b-q4.gguf".
	Model string

	// ContextSize is the model context window (tokens). Default: 4096.
	ContextSize int

	// Threads is CPU thread count for llama.cpp subprocess. Default: runtime.NumCPU()/2.
	Threads int

	// GPULayers offloads N layers to GPU (0 = CPU only). Default: 0.
	GPULayers int

	// Port for llama.cpp HTTP server (0 = ephemeral).
	Port int

	// StartupTimeout is how long to wait for subprocess health-check.
	StartupTimeout time.Duration
}

// Request is a single inference request.
type Request struct {
	// Prompt is the raw input text (caller handles any chat templating).
	Prompt string

	// MaxTokens caps output length.
	MaxTokens int

	// Temperature for sampling (0.0 = deterministic, 0.7 = balanced, >1 = creative).
	Temperature float64

	// TopP for nucleus sampling (0.0-1.0). Default: 0.9.
	TopP float64

	// Stop sequences — generation ends on any match.
	Stop []string

	// IsGreenfield distinguishes Healer gate tier per Gemini rc120 spec:
	//   - false (default) = SurgicalGate 2s — patch existing (syntax/import fix)
	//   - true            = GreenfieldGate 10s — generate new block from scratch
	//
	// Caller set true untuk new-function/new-file generation, false untuk
	// patch-existing-code. Impact: ctx deadline di Stream dihitung dari flag.
	IsGreenfield bool
}

// Healer gate deadlines per Gemini rc120 clarification.
const (
	SurgicalGate   = 2 * time.Second
	GreenfieldGate = 10 * time.Second
)

// Response wraps generated text + timing.
type Response struct {
	// Text is the generated completion.
	Text string

	// FinishReason: "stop" / "length" / "timeout" / "abort".
	FinishReason string

	// PromptTokens + CompletionTokens for accounting (local = $0 but logged).
	PromptTokens     int
	CompletionTokens int

	// TotalMS is wall-clock latency — critical untuk Healer <2s gate.
	TotalMS int64
}

// Status reports runtime health.
type Status struct {
	Ready      bool
	Backend    Backend
	Model      string
	LastErr    string
	SubprocPID int // 0 untuk PureGo
	UptimeMS   int64
}

// ErrNotReady means runtime belum initialized atau sudah Close.
var ErrNotReady = errors.New("localai: runtime not ready")

// ErrModelMissing means GGUF file tidak ditemukan di ~/.flowork/models/.
var ErrModelMissing = errors.New("localai: model file missing — run `flowork model pull <name>`")

// ErrHealerTimeout means inference exceeded Healer gate deadline.
// Per Gemini rc120 clarification: 2s SurgicalGate atau 10s GreenfieldGate
// (tergantung Request.IsGreenfield flag).
//
// Caller flow on ErrHealerTimeout:
//   - Surgical: 1x retry queue sebelum Fatal Decoherence rollback.
//   - Greenfield: fallback ke Evolution-tier cloud call (dengan BudgetGuard).
var ErrHealerTimeout = errors.New("localai: healer gate deadline exceeded")

// Open creates a new Runtime. Phase A (llamacpp) is the only Backend
// implemented initially. BackendPureGo akan diwire di Sprint 4+.
func Open(cfg Config) (Runtime, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("localai: Config.Model required")
	}
	if cfg.ContextSize == 0 {
		cfg.ContextSize = 4096
	}
	if cfg.StartupTimeout == 0 {
		cfg.StartupTimeout = 30 * time.Second
	}

	backend := cfg.Backend
	if backend == "" || backend == BackendAuto {
		backend = BackendLlamaCpp // Phase A default until BackendPureGo ready
	}

	switch backend {
	case BackendLlamaCpp:
		return newLlamaCppRuntime(cfg)
	case BackendPureGo:
		return nil, errors.New("localai: BackendPureGo not yet implemented (Sprint 4+)")
	default:
		return nil, fmt.Errorf("localai: unknown backend %q", backend)
	}
}
