// Per-IDE MITM handler base.

package handlers

import (
	"net/http"
	"sync"
)

// Handler is the per-IDE contract. Handle receives the original request and
// must call ServeHTTP back with a rewritten one OR write the response itself.
type Handler interface {
	Name() string
	Handle(w http.ResponseWriter, r *http.Request)
}

var (
	regMu    sync.RWMutex
	registry = map[string]Handler{}
)

// Register adds a handler. Called from init() of each per-IDE file.
func Register(h Handler) {
	if h == nil || h.Name() == "" {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	registry[h.Name()] = h
}

// Get returns a handler by name (e.g. "antigravity", "copilot", "cursor",
// "kiro"), or nil when none.
func Get(name string) Handler {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[name]
}

// List returns registered names (sorted not guaranteed).
func List() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
