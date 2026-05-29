// Package policy — Pilar 9 Infrastructure v0.5+ PolicyLimits.
//
// Baca konfigurasi per-model / per-agent dari `~/.flowork/policy.yaml`
// (atau `.flowork-local/policy.yaml` workspace-portable). Pattern mirip
// SSH config: baseline rule di atas, override specific di bawah.
//
// File format (YAML):
//
//	defaults:
//	  daily_usd_cap: 5.00
//	  per_task_usd_cap: 0.20
//	  max_tokens: 4096
//	  allow_free_fallback: true
//
//	models:
//	  "anthropic/claude-opus-4-7":
//	    daily_usd_cap: 3.00
//	    per_task_usd_cap: 0.50    # Opus mahal, cap per-task lebih kencang dari default
//	  "deepseek/deepseek-chat":
//	    daily_usd_cap: 0.50        # cheap tier — kasih ruang
//
//	agents:
//	  watcher:
//	    daily_usd_cap: 0.00        # watcher WAJIB free-only
//	    force_free: true
//	  telegram:
//	    daily_usd_cap: 2.00
//	  opus-3:
//	    daily_usd_cap: 3.00
//
// Loader ini pure reader — pure-Go YAML via `gopkg.in/yaml.v3` yang sudah
// dipakai di `internal/config`. Zero dep tambahan.
package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Limits adalah kontrak policy per-entity. Field zero = "tidak di-set"
// (biar layering bisa deteksi kapan fallback ke default).
type Limits struct {
	DailyUSDCap       float64 `yaml:"daily_usd_cap,omitempty"`
	PerTaskUSDCap     float64 `yaml:"per_task_usd_cap,omitempty"`
	MaxTokens         int     `yaml:"max_tokens,omitempty"`
	AllowFreeFallback *bool   `yaml:"allow_free_fallback,omitempty"`
	ForceFree         bool    `yaml:"force_free,omitempty"`
	// Tags bebas buat routing hint (mis. "read-only", "high-context").
	Tags []string `yaml:"tags,omitempty"`
}

// Policy adalah root document.
type Policy struct {
	Defaults Limits            `yaml:"defaults"`
	Models   map[string]Limits `yaml:"models,omitempty"`
	Agents   map[string]Limits `yaml:"agents,omitempty"`
}

// searchPaths mengurutkan lokasi yang dicek — workspace-portable dulu,
// baru fallback ke home folder. Hasil pertama yang exist dipakai.
func searchPaths(workspace string) []string {
	var out []string
	if workspace != "" {
		out = append(out,
			filepath.Join(workspace, ".flowork-local", "policy.yaml"),
			filepath.Join(workspace, "policy.yaml"),
		)
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out,
			filepath.Join(home, ".flowork", "policy.yaml"),
		)
	}
	return out
}

// Load baca file pertama yang ketemu. Return Policy kosong + error nil
// kalau tidak ada file (caller boleh interpretasi: "pakai default saja").
func Load(workspace string) (*Policy, string, error) {
	for _, p := range searchPaths(workspace) {
		raw, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, p, fmt.Errorf("read policy %q: %w", p, err)
		}
		var pol Policy
		if err := yaml.Unmarshal(raw, &pol); err != nil {
			return nil, p, fmt.Errorf("parse policy %q: %w", p, err)
		}
		return &pol, p, nil
	}
	return &Policy{}, "", nil
}

// MergeLimits layer defaults → model-specific → agent-specific.
// Agent override dimenangkan karena lebih specific dari model.
// Zero-value di layer atas tidak overwrite nilai yang sudah ada di bawah.
func (p *Policy) MergeLimits(model, agent string) Limits {
	merged := p.Defaults
	if model != "" {
		if ml, ok := p.Models[model]; ok {
			merged = overlay(merged, ml)
		}
	}
	if agent != "" {
		if al, ok := p.Agents[agent]; ok {
			merged = overlay(merged, al)
		}
	}
	return merged
}

func overlay(base, override Limits) Limits {
	out := base
	if override.DailyUSDCap > 0 {
		out.DailyUSDCap = override.DailyUSDCap
	}
	if override.PerTaskUSDCap > 0 {
		out.PerTaskUSDCap = override.PerTaskUSDCap
	}
	if override.MaxTokens > 0 {
		out.MaxTokens = override.MaxTokens
	}
	if override.AllowFreeFallback != nil {
		out.AllowFreeFallback = override.AllowFreeFallback
	}
	if override.ForceFree {
		out.ForceFree = true
	}
	if len(override.Tags) > 0 {
		// Merge: unique union.
		seen := map[string]bool{}
		for _, t := range out.Tags {
			seen[t] = true
		}
		for _, t := range override.Tags {
			if !seen[t] {
				out.Tags = append(out.Tags, t)
				seen[t] = true
			}
		}
	}
	return out
}

// ── process-level cached loader ────────────────────────────────────────

var (
	sharedMu   sync.RWMutex
	sharedPol  *Policy
	sharedPath string
	sharedTS   time.Time
)

// Shared return cached singleton. Cache invalidated tiap 60 detik kalau
// file di-modify — supaya Ayah bisa edit policy.yaml tanpa restart binary.
func Shared(workspace string) *Policy {
	sharedMu.RLock()
	pol, path, ts := sharedPol, sharedPath, sharedTS
	sharedMu.RUnlock()

	if pol != nil && time.Since(ts) < 60*time.Second {
		return pol
	}

	fresh, newPath, err := Load(workspace)
	if err != nil {
		// Parse error — log ke stderr, pertahankan cached version supaya
		// misconfig YAML tidak langsung lumpuhkan budget enforcement.
		fmt.Fprintf(os.Stderr, "policy: load error (%s): %v — kept previous cache\n", newPath, err)
		if pol != nil {
			return pol
		}
		fresh = &Policy{} // degrade ke empty defaults
	}

	sharedMu.Lock()
	sharedPol = fresh
	sharedPath = newPath
	sharedTS = time.Now()
	sharedMu.Unlock()

	_ = path // reserved for future invalidation diff
	return fresh
}
