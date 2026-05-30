// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Search dispatcher (multi-provider normalizer).

package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Request is the canonical search shape used by /v1/search.
type Request struct {
	Query      string
	MaxResults int
	APIKey     string
	BaseURL    string
	Extra      map[string]any
}

// Result is the normalized envelope returned to the caller.
type Result struct {
	Provider string         `json:"provider"`
	Query    string         `json:"query"`
	Results  []SearchResult `json:"results"`
}

// SearchResult is one item.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

// SearchProvider is the vendor contract.
type SearchProvider interface {
	Name() string
	Search(ctx context.Context, req Request) (*Result, error)
}

var (
	regMu    sync.RWMutex
	registry = map[string]SearchProvider{}
)

// Register adds a provider.
func Register(p SearchProvider) {
	if p == nil || p.Name() == "" {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	registry[p.Name()] = p
}

// Get returns a provider by name, or nil.
func Get(name string) SearchProvider {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[name]
}

// List returns every registered name.
func List() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

// ── shared helpers ──────────────────────────────────────────────────────

var searchHTTPClient = &http.Client{Timeout: 30 * time.Second}

func doRequest(req *http.Request, into any) error {
	resp, err := searchHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream %d: %s", resp.StatusCode, head(body))
	}
	return json.Unmarshal(body, into)
}

func head(b []byte) string {
	if len(b) > 240 {
		return string(b[:240]) + "…"
	}
	return string(b)
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
