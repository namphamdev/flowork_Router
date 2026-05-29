// Package proxy mengimplementasikan FQ-Brain sebagai provider.Client proxy.
//
// Proxy ini duduk di antara FloworkOS dan OpenRouter:
//   - Shadow mode (default): forward semua, record response untuk training
//   - Proxy mode: check confidence, jawab sendiri jika tinggi
//   - Selalu record: setiap interaksi disimpan ke SQLite
//
// Implements provider.Client interface sehingga plug-and-play ke fallback chain.
//
// Env vars:
//   - FQBRAIN_ENABLED=1   → aktifkan proxy (default: 0/off)
//   - FQBRAIN_SHADOW=0    → proxy mode (default: 1/shadow)
//   - FQBRAIN_VERBOSE=1   → verbose logging ke stderr
//   - FQBRAIN_DB_PATH     → custom path ke sqlite (default: brain/flowork-brain.sqlite)
package proxy

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/teetah2402/flowork/brain/db"
	"github.com/teetah2402/flowork/brain/ingestor"
	brainv2 "github.com/teetah2402/flowork/internal/brain"
	"github.com/teetah2402/flowork/internal/provider"
	"github.com/teetah2402/flowork/internal/wellness"
)

// ProxyConfig mengonfigurasi perilaku brain proxy.
type ProxyConfig struct {
	// ShadowMode — jika true, proxy TIDAK pernah jawab sendiri.
	// Hanya record & compare. Default: true (aman).
	ShadowMode bool

	// ConfidenceThreshold — minimum confidence untuk jawab sendiri.
	// Hanya relevan jika ShadowMode=false. Range: 0.0-1.0.
	ConfidenceThreshold float64

	// RecordToolCalls — jika true, record tool_calls beserta arguments.
	RecordToolCalls bool

	// DecayRate — multiplier decay amplitude per cycle. Default: 0.995.
	DecayRate float64

	// Verbose — log setiap interaksi ke stderr.
	Verbose bool
}

// DefaultConfig mengembalikan konfigurasi default (M11 cascade activation).
//
// 2026-04-30 (M11.1): default ShadowMode FLIP dari true → false. Cascade
// sekarang ke-fire di prod (L1 cache + L2 hybrid + L4 peer hit-check sebelum
// forward upstream OpenRouter). Override via env FQBRAIN_SHADOW=1 (legacy
// safe-mode) atau settings DB key BRAIN_CASCADE_SHADOW_MODE=true.
//
// Goal: ≥60-80% query 0 API call dalam 1 minggu setelah flip (sovereignty
// telemetry tracking via internal/wellness/sovereignty.go).
func DefaultConfig() ProxyConfig {
	// Default false = cascade aktif. Env=1 atau "true" = shadow mode (legacy).
	shadowMode := false
	if v := strings.TrimSpace(os.Getenv("FQBRAIN_SHADOW")); v == "1" || strings.EqualFold(v, "true") {
		shadowMode = true
	}
	return ProxyConfig{
		ShadowMode:          shadowMode,
		ConfidenceThreshold: 0.95,
		RecordToolCalls:     true,
		DecayRate:           0.995,
		Verbose:             strings.TrimSpace(os.Getenv("FQBRAIN_VERBOSE")) == "1",
	}
}

// BrainProxy adalah provider.Client yang membungkus upstream provider
// dengan layer recording dan (optional) self-answering.
// rc189: parasite ingestor dropped (atoms+entanglements zombie write data).
type BrainProxy struct {
	db          *sql.DB
	upstream    provider.Client
	config      ProxyConfig
	recorder    *Recorder
	toolLearner *ingestor.ToolLearner

	mu    sync.RWMutex
	stats ProxyStats
}

// ProxyStats melacak statistik proxy.
type ProxyStats struct {
	TotalRequests   int64
	Forwarded       int64
	SelfAnswered    int64
	RecordingErrors int64
}

// NewBrainProxy membuat proxy baru.
// upstream = provider yang akan menerima forwarded requests (OpenRouter).
func NewBrainProxy(db *sql.DB, upstream provider.Client, cfg ProxyConfig) *BrainProxy {
	return &BrainProxy{
		db:          db,
		upstream:    upstream,
		config:      cfg,
		recorder:    NewRecorder(db),
		toolLearner: ingestor.NewToolLearner(db),
	}
}

// Name — implement provider.Client
func (p *BrainProxy) Name() string {
	return "fq-brain"
}

// Complete — implement provider.Client
//
// Flow:
//  1. Shadow mode → always forward, record response
//  2. Proxy mode → check confidence → self-answer OR forward
//  3. Always record interaction ke recordings table
func (p *BrainProxy) Complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	p.mu.Lock()
	p.stats.TotalRequests++
	p.mu.Unlock()

	// Extract prompt text untuk recording
	promptText := extractPromptText(req)

	// Shadow mode: selalu forward
	if p.config.ShadowMode {
		return p.forwardAndRecord(ctx, req, promptText)
	}

	// Proxy mode — Reasoning Cascade L1-L4 sebelum LLM external.
	// L5 (LLM call) disengaja diserahkan ke forwardAndRecord supaya recordings +
	// parasite ingest tetap jalan via path original (bukan sekadar cache write).
	cascadeCfg := brainv2.DefaultCascadeConfig()
	cascadeCfg.StoreNewResponses = false // forwardAndRecord yang nyimpen
	res, cerr := brainv2.Resolve(ctx, p.db, promptText, p.upstream.Name(), cascadeCfg, nil)
	if cerr == nil && res.Layer != "" && res.Layer != "L5-llm" {
		// Cache / similar / KG hit — TIDAK perlu hit OpenRouter.
		p.mu.Lock()
		p.stats.SelfAnswered++
		p.mu.Unlock()
		// M11.3: track sovereignty score (cascade hit, no upstream call)
		wellness.RecordCascadeHit(res.Layer)
		if p.config.Verbose {
			log.Printf("fq-brain: cascade hit %s score=%.2f tokens_saved=%.0f latency=%dms",
				res.Layer, res.Score, res.TokenSaved, res.LatencyMs)
		}
		return provider.Response{
			Message: provider.Message{
				Role:    provider.RoleAssistant,
				Content: res.Response,
			},
		}, nil
	}

	// Cascade miss → forward to upstream. rc189: confidence telemetry dropped
	// (atom-coverage scoring depended on zombie atoms/entanglements data).
	// M11.3: track sovereignty score (upstream call)
	wellness.RecordUpstreamCall()
	if p.config.Verbose {
		log.Printf("fq-brain: cascade miss → forwarding")
	}
	return p.forwardAndRecord(ctx, req, promptText)
}

// forwardAndRecord forward request ke upstream dan record hasilnya.
func (p *BrainProxy) forwardAndRecord(ctx context.Context, req provider.Request, promptText string) (provider.Response, error) {
	resp, err := p.upstream.Complete(ctx, req)
	if err != nil {
		return resp, err
	}

	p.mu.Lock()
	p.stats.Forwarded++
	p.mu.Unlock()

	// Fire-and-forget recording + learning — jangan block response
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Recorder panic gak boleh nge-crash proxy.
				log.Printf("brain/proxy: recorder panic recovered: %v", r)
			}
		}()
		responseText := resp.Message.Content

		// 1. Record interaksi ke recordings table
		if recErr := p.recorder.RecordInteraction(
			promptText,
			responseText,
			req.Model,
			resp.Usage.InputTokens,
			resp.Usage.OutputTokens,
			resp.Message.ToolCalls,
			p.upstream.Name(),
		); recErr != nil {
			p.mu.Lock()
			p.stats.RecordingErrors++
			p.mu.Unlock()
			if p.config.Verbose {
				log.Printf("fq-brain: recording error: %v", recErr)
			}
			return // skip learning jika recording gagal
		}

		// 1.5 Tier 1.4 Build Verifier (post opus-3 bug fix 2026-05-10):
		// classify response sebagai pass/fail via heuristic + DB-backed config,
		// call MarkBuildResult. Anti cascading idle (skillminer, tier-promoter,
		// finetune pipeline yang gate via build_pass=1).
		// Idempotent: MarkBuildResult guard WHERE build_pass=-1.
		p.recorder.VerifyAndMark(promptText, responseText, resp.Message.ToolCalls)

		// rc189: parasite trigram ingest dropped (atoms+entanglements zombie data).

		// 2. Tool pattern learning — belajar kapan pakai tool apa
		if len(resp.Message.ToolCalls) > 0 {
			p.toolLearner.LearnFromToolCalls(promptText, resp.Message.ToolCalls)

			// Juga record individual tool calls via recorder
			if p.config.RecordToolCalls {
				for _, tc := range resp.Message.ToolCalls {
					p.recorder.RecordToolCall(promptText, tc.Name, string(tc.Arguments), true)
				}
			}
		}

		// 4. Cache reasoning — investasi permanen. Next call dengan prompt
		//    yang sama bakal hit Layer 1 cascade tanpa burning OpenRouter token.
		//    Skip kalau response kosong (e.g. error / empty completion).
		if responseText != "" && len(resp.Message.ToolCalls) == 0 {
			if _, err := brainv2.CacheResponse(p.db, promptText, responseText, req.Model, p.upstream.Name(), ""); err != nil {
				if p.config.Verbose {
					log.Printf("fq-brain: cache write error: %v", err)
				}
			}
		}
	}()

	return resp, nil
}

// Stats mengembalikan statistik proxy saat ini.
func (p *BrainProxy) Stats() ProxyStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// extractPromptText mengekstrak teks dari request messages.
func extractPromptText(req provider.Request) string {
	var parts []string
	for _, msg := range req.Messages {
		if msg.Role == provider.RoleUser && msg.Content != "" {
			parts = append(parts, msg.Content)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	// Ambil pesan user terakhir sebagai prompt utama
	return parts[len(parts)-1]
}

// SetShadowMode mengubah mode proxy secara runtime.
func (p *BrainProxy) SetShadowMode(shadow bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.ShadowMode = shadow
	mode := "SHADOW"
	if !shadow {
		mode = "PROXY"
	}
	fmt.Fprintf(os.Stderr, "fq-brain: switched to %s mode\n", mode)
}

// ──────────────────────────────────────────────────────────────────────────
// WrapWithBrain — fungsi utama untuk integrasi ke fallback chain.
//
// Cara pakai di mana saja yang create provider.Client:
//
//	client, model, err := buildClient(cfg)
//	client = proxy.WrapWithBrain(client, workspace) // otomatis cek env
//
// Env vars yang dibaca:
//
//	FQBRAIN_ENABLED=1  → wrap client dengan BrainProxy
//	FQBRAIN_SHADOW=0   → aktifkan proxy mode (default shadow)
//	FQBRAIN_VERBOSE=1  → verbose logging
//	FQBRAIN_DB_PATH    → custom sqlite path (default: <workspace>/brain/flowork-brain.sqlite)
//
// Jika FQBRAIN_ENABLED != "1", fungsi ini mengembalikan client tanpa modifikasi.
// ──────────────────────────────────────────────────────────────────────────
func WrapWithBrain(upstream provider.Client, workspace string) provider.Client {
	if strings.TrimSpace(os.Getenv("FQBRAIN_ENABLED")) != "1" {
		return upstream
	}

	dbPath := strings.TrimSpace(os.Getenv("FQBRAIN_DB_PATH"))
	if dbPath == "" {
		dbPath = filepath.Join(workspace, "brain", "flowork-brain.sqlite")
	}

	// Fix bug-9: daemon cc/aksara/wiraga/dll boot bareng → race condition
	// DDL kalau semua panggil InitDB bersamaan. Helper ini assume GUI/brain
	// sudah inisialisasi schema saat boot; daemon cukup Open koneksi biasa.
	database, err := db.Open(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fq-brain: ⚠️ gagal buka DB %s: %v — lanjut TANPA proxy\n", dbPath, err)
		return upstream
	}

	cfg := DefaultConfig()
	mode := "SHADOW"
	if !cfg.ShadowMode {
		mode = "PROXY"
	}

	fmt.Fprintf(os.Stderr, "fq-brain: ✅ AKTIF mode=%s wrapping=%s db=%s\n", mode, upstream.Name(), dbPath)

	return NewBrainProxy(database, upstream, cfg)
}

// Close releases the database handle. Dipanggil oleh daemon shutdown path
// supaya koneksi fq-brain proxy tidak jadi leak saat Ctrl+C.
func (p *BrainProxy) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.recorder != nil {
		if closer, ok := any(p.recorder).(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}
	// recorder typically stores the *sql.DB — kalau struct-nya expose Close()
	// kita honor, else nothing to do. Daemon lifecycle biasanya short-lived.
	return nil
}
