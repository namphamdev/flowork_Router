// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider adapter.

// Embedding provider catalog.

package embedding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Request is the canonical embedding request.
type Request struct {
	Model      string
	Input      []string
	APIKey     string
	BaseURL    string
	Dimensions int
}

// Result is the OpenAI-shape embeddings response.
type Result struct {
	Object string  `json:"object"`
	Data   []Embed `json:"data"`
	Model  string  `json:"model"`
	Usage  Usage   `json:"usage"`
}

type Embed struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type Usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// EmbeddingProvider is the vendor contract.
type EmbeddingProvider interface {
	Name() string
	Embed(ctx context.Context, req Request) (*Result, error)
}

var (
	regMu    sync.RWMutex
	registry = map[string]EmbeddingProvider{}
)

// Register adds a provider.
func Register(p EmbeddingProvider) {
	if p == nil || p.Name() == "" {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	registry[p.Name()] = p
}

// Get returns a provider by name, or nil.
func Get(name string) EmbeddingProvider {
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

// ── shared helpers ────────────────────────────────────────────────────────

var embedHTTPClient = &http.Client{Timeout: 2 * time.Minute}

func doEmbedRequest(r *http.Request) (*Result, error) {
	resp, err := embedHTTPClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, head(body))
	}
	var out Result
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if out.Object == "" {
		out.Object = "list"
	}
	return &out, nil
}

func head(b []byte) string {
	if len(b) > 240 {
		return string(b[:240]) + "…"
	}
	return string(b)
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
