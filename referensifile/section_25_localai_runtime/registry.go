// Brain V3 Phase 5: Specialized Models per Warga + LRU Pool.
//
// Registry maps warga (atau task type) → gguf model filename.
// Pool manage multiple Runtime concurrent dengan LRU eviction supaya RAM
// gak meledak saat banyak model registered.
//
// Default mapping (rc188 — Ayah swap ke single Qwen3.6-27B 2026-04-26):
//
//	semua warga → Qwen3.6-27B-Q4_K_M (~16GB) — model lokal SATU-SATUNYA yang
//	ke-download di .flowork/models/. Specialized per-warga model (Phi/Deepseek/
//	Qwen-Coder) di-defer sampai Ayah download model spesifik.
//
// Override per warga via DB (settings tabel atau agents.model = "local:..").
//
// LRU: max 1 model loaded (Qwen 27B = 16GB RAM, single slot cukup; pool bisa
// scale up nanti kalau model lain ke-download).
package localai

import (
	"sync"
	"time"
)

// ModelSpec — 1 entri di registry.
type ModelSpec struct {
	GGUF     string // filename gguf di ~/.flowork/models/
	Template string // chat template hint untuk LocalLlamaClient (auto-detect kalau empty)
	Notes    string // dokumentasi free-form (siapa pake, tier hardware)
	SizeMB   int    // approximate disk + RAM footprint
}

// DefaultGGUF — single model name yang ke-download di
// <project_root>/models/. Sebelumnya Qwen3.6-27B (16GB hybrid offload)
// tapi dependency-nya berat. Ganti ke Qwen 2.5 3B Instruct abliterated
// (5.8GB F16) yang udah di-convert dari HF safetensors + portable di
// project root (RULE EMAS §1.4). CPU-friendly via llama-server.exe
// prebuilt (bin/llamacpp/). Override via env FLOWORK_LOCAL_MODEL atau
// FLOWORK_INFERENCE_GGUF.
const DefaultGGUF = "qwen2.5-3b-abliterated-f16.gguf"

// defaultMap — production mapping. rc188: semua warga ke single Qwen 27B
// karena cuma itu gguf yang lokal. Future: re-add specialized models pas
// Ayah download Phi-3/DeepSeek/Qwen-Coder.
var defaultMap = map[string]ModelSpec{
	"merpati": {
		GGUF:     DefaultGGUF,
		Template: "chatml",
		Notes:    "Chat owner Telegram. Pakai Qwen 27B (rc188).",
		SizeMB:   16500,
	},
	"aksara": {
		GGUF:     DefaultGGUF,
		Template: "chatml",
		Notes:    "Code generation, refactor.",
		SizeMB:   16500,
	},
	"wiraga": {
		GGUF:     DefaultGGUF,
		Template: "chatml",
		Notes:    "Heavy refactor + architecture.",
		SizeMB:   16500,
	},
	"selam": {
		GGUF:     DefaultGGUF,
		Template: "chatml",
		Notes:    "Deep reasoning specialty.",
		SizeMB:   16500,
	},
	"default": {
		GGUF:     DefaultGGUF,
		Template: "chatml",
		Notes:    "Generalist fallback. Qwen3.6-27B Q4 (~16GB hybrid offload).",
		SizeMB:   16500,
	},
}

// Registry — thread-safe wrapper buat warga→spec mapping. Bisa di-override
// programmatic atau di-load dari DB warga_model_map (future).
type Registry struct {
	mu       sync.RWMutex
	mappings map[string]ModelSpec
}

// NewRegistry bikin registry dengan default mapping.
func NewRegistry() *Registry {
	out := make(map[string]ModelSpec, len(defaultMap))
	for k, v := range defaultMap {
		out[k] = v
	}
	return &Registry{mappings: out}
}

// Lookup return ModelSpec untuk warga. Fallback chain:
//
//	1. exact match warga
//	2. "default" entry
//	3. zero-value spec + ok=false
func (r *Registry) Lookup(warga string) (ModelSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if spec, ok := r.mappings[warga]; ok {
		return spec, true
	}
	if spec, ok := r.mappings["default"]; ok {
		return spec, true
	}
	return ModelSpec{}, false
}

// Override set/replace mapping untuk satu warga. Buat runtime tweak —
// permanent change harus persist via DB.
func (r *Registry) Override(warga string, spec ModelSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mappings[warga] = spec
}

// Remove drop mapping untuk warga (revert ke "default" lookup).
func (r *Registry) Remove(warga string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.mappings, warga)
}

// All return snapshot semua mappings (read-only copy).
func (r *Registry) All() map[string]ModelSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]ModelSpec, len(r.mappings))
	for k, v := range r.mappings {
		out[k] = v
	}
	return out
}

// ─── LRU Pool untuk Runtime ───────────────────────────────────────────────

// runtimeSlot — 1 cached runtime entry.
type runtimeSlot struct {
	gguf     string
	rt       Runtime
	lastUsed time.Time
}

// Pool manage multiple Runtime concurrent dengan LRU eviction. Max N model
// loaded simultaneously — saat add ke-(N+1), evict (Close) yang lastUsed
// paling lama.
//
// Thread-safe. Usage:
//
//	pool := NewPool(3)
//	rt, err := pool.Acquire(cfg)  // create or hit cache
//	... use rt ...
//	(no explicit release — LRU evict handle lifecycle)
type Pool struct {
	mu       sync.Mutex
	maxSlots int
	slots    map[string]*runtimeSlot
}

// NewPool bikin pool dengan max N concurrent runtimes.
// rc188: default 1 slot (Qwen 27B = 16GB; multi-slot perlu RAM 32GB+).
// Caller bisa override naikin slot kalau ada model kecil yang ke-download.
func NewPool(maxSlots int) *Pool {
	if maxSlots <= 0 {
		maxSlots = 1
	}
	return &Pool{
		maxSlots: maxSlots,
		slots:    make(map[string]*runtimeSlot),
	}
}

// Acquire return Runtime untuk gguf. Cache hit kalau already loaded; else
// Open new (potentially evict LRU). Bump lastUsed before return.
func (p *Pool) Acquire(cfg Config) (Runtime, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if slot, ok := p.slots[cfg.Model]; ok {
		slot.lastUsed = time.Now()
		return slot.rt, nil
	}

	if len(p.slots) >= p.maxSlots {
		p.evictLRU()
	}

	rt, err := Open(cfg)
	if err != nil {
		return nil, err
	}
	p.slots[cfg.Model] = &runtimeSlot{
		gguf:     cfg.Model,
		rt:       rt,
		lastUsed: time.Now(),
	}
	return rt, nil
}

// evictLRU drop slot dengan lastUsed paling lama. Caller harus pegang p.mu.
func (p *Pool) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, s := range p.slots {
		if first || s.lastUsed.Before(oldestTime) {
			oldestKey = k
			oldestTime = s.lastUsed
			first = false
		}
	}
	if oldestKey != "" {
		_ = p.slots[oldestKey].rt.Close()
		delete(p.slots, oldestKey)
	}
}

// CloseAll evict semua. Berguna pas shutdown daemon.
func (p *Pool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, s := range p.slots {
		_ = s.rt.Close()
		delete(p.slots, k)
	}
}

// Stats snapshot pool state — count + per-model lastUsed age.
func (p *Pool) Stats() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]any{
		"slots_used": len(p.slots),
		"slots_max":  p.maxSlots,
	}
	models := make([]map[string]any, 0, len(p.slots))
	for _, s := range p.slots {
		models = append(models, map[string]any{
			"model":          s.gguf,
			"last_used_unix": s.lastUsed.Unix(),
			"idle_seconds":   int64(time.Since(s.lastUsed).Seconds()),
		})
	}
	out["models"] = models
	return out
}
