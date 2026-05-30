// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Live quota fetchers.

package quotalive

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Window is one quota dimension (e.g. 5-hour rolling window, weekly cap).
// Used + Total + Remaining are in the same unit and same scale; for percent
// quotas we expose Used as the percentage and Total = 100.
type Window struct {
	Label            string    `json:"label"`
	Used             float64   `json:"used"`
	Total            float64   `json:"total"`
	Remaining        float64   `json:"remaining"`
	RemainingPercent float64   `json:"remainingPercent"`
	ResetAt          time.Time `json:"resetAt,omitempty"`
	Unlimited        bool      `json:"unlimited,omitempty"`
	Unit             string    `json:"unit,omitempty"` // "requests" | "tokens" | "percent"
}

// Snapshot is the whole picture for one provider — multiple Windows + plan
// name + fetched-at timestamp.
type Snapshot struct {
	Provider  string    `json:"provider"`
	Plan      string    `json:"plan,omitempty"`
	FetchedAt time.Time `json:"fetchedAt"`
	Windows   []Window  `json:"windows"`
	Raw       []byte    `json:"-"` // upstream JSON for debugging; omitted from API
}

// Params is the per-call config a Fetcher needs. Token is the upstream
// credential; ProviderID is the provider record (so the fetcher can stash
// custom Data fields if needed).
type Params struct {
	Token      string
	ProviderID string
	Extra      map[string]any
}

// LiveFetcher is the vendor contract. Implementations should not cache —
// caching is the caller's concern (the route handler chooses when to refresh).
type LiveFetcher interface {
	Name() string
	Fetch(ctx context.Context, p Params) (Snapshot, error)
}

var (
	regMu    sync.RWMutex
	registry = map[string]LiveFetcher{}
)

// Register adds a fetcher (idempotent — last writer wins).
func Register(f LiveFetcher) {
	if f == nil || f.Name() == "" {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	registry[f.Name()] = f
}

// Get returns the fetcher by name, or nil.
func Get(name string) LiveFetcher {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[name]
}

// List returns every registered vendor name.
func List() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

// httpClient is shared across fetchers.
var httpClient = &http.Client{Timeout: 30 * time.Second}
