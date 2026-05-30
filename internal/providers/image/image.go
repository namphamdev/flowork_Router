// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider adapter.

// Image provider catalog (12 vendor).

package image

import (
	"context"
	"sync"
)

// Request is the minimum image-gen shape every vendor accepts. The caller
// constructs this from /v1/images JSON; vendor code shapes it into upstream.
type Request struct {
	Model          string
	Prompt         string
	NegativePrompt string
	Size           string // e.g. "1024x1024"
	N              int    // count
	Quality        string // e.g. "standard" / "hd"
	Style          string
	APIKey         string
	BaseURL        string // optional override
	Extra          map[string]any
}

// Result is the normalized OpenAI-shape response (one or more URLs / b64).
type Result struct {
	Data []ResultImage `json:"data"`
}

type ResultImage struct {
	URL     string `json:"url,omitempty"`
	B64JSON string `json:"b64_json,omitempty"`
}

// ImageProvider is the vendor contract.
type ImageProvider interface {
	Name() string
	Generate(ctx context.Context, req Request) (*Result, error)
}

var (
	regMu    sync.RWMutex
	registry = map[string]ImageProvider{}
)

// Register adds a provider. Called from init() of each vendor file.
func Register(p ImageProvider) {
	if p == nil || p.Name() == "" {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	registry[p.Name()] = p
}

// Get returns a provider by name, or nil.
func Get(name string) ImageProvider {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[name]
}

// List returns every registered name (sort order not guaranteed).
func List() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
