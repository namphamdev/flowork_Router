// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Web-fetch provider catalog.

package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Request is the URL-fetch shape. Mode is vendor-specific hint (e.g. "raw" /
// "markdown" / "screenshot") and may be ignored by simpler vendors.
type Request struct {
	URL     string
	Mode    string
	APIKey  string
	BaseURL string
	Extra   map[string]any
}

// Result is the vendor-neutral response. ContentType reflects the actual MIME
// of Body (text/markdown for reader services, text/html for raw fetch).
type Result struct {
	URL         string
	Title       string
	Body        []byte
	ContentType string
	StatusCode  int
}

// Fetcher is the vendor contract.
type Fetcher interface {
	Name() string
	Fetch(ctx context.Context, req Request) (Result, error)
}

var (
	regMu    sync.RWMutex
	registry = map[string]Fetcher{}
)

// Register adds a provider (idempotent — last writer wins).
func Register(p Fetcher) {
	if p == nil || p.Name() == "" {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	registry[p.Name()] = p
}

// Get returns the provider by name, or nil.
func Get(name string) Fetcher {
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

var fetchHTTPClient = &http.Client{Timeout: 60 * time.Second}

// doHTTPRequest sends r and reads up to 8 MiB. Caller-supplied headers are
// already on the request.
func doHTTPRequest(r *http.Request) ([]byte, *http.Response, error) {
	resp, err := fetchHTTPClient.Do(r)
	if err != nil {
		return nil, nil, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	return body, resp, nil
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
