// Vendor Executor Framework.

package executors

import (
	"context"
	"net/http"
	"sync"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// Usage mirrors router.OpenAIUsage but stays import-cycle-free.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Request is a minimal subset of router.OpenAIRequest that executors care
// about. Keeping it small avoids pulling the router package into here (which
// would create an import cycle, since the dispatcher imports executors).
type Request struct {
	Model       string
	Messages    []Message
	MaxTokens   int
	Temperature float64
	TopP        float64
	Stream      bool
	Tools       []map[string]any
	RawJSON     []byte // when not empty, executor MAY use as-is (translator already produced vendor shape)
}

type Message struct {
	Role    string
	Content string
}

// Executor is the vendor-specific contract. Stream MUST write SSE chunks to w
// in OpenAI delta shape (so the rest of the pipeline stays format-neutral).
// NonStream returns a fully-formed OpenAI ChatCompletion response (JSON bytes
// and metadata) — keep it lightweight: the dispatcher unmarshals.
type Executor interface {
	Name() string
	Stream(ctx context.Context, p *store.ProviderConnection, req Request, w http.ResponseWriter, flusher http.Flusher) (Usage, int, error)
	NonStream(ctx context.Context, p *store.ProviderConnection, req Request) (respBody []byte, usage Usage, status int, err error)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Executor{}
)

// Register adds an Executor to the registry. Last write wins so a sub-package
// may override an earlier built-in. Called from init() in each executor file.
func Register(e Executor) {
	if e == nil || e.Name() == "" {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[e.Name()] = e
}

// Get returns the executor for name, or nil when none is registered.
func Get(name string) Executor {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[name]
}

// List returns the names of all registered executors (sorted not guaranteed).
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
