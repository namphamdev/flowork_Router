// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider request/response translator.

// Translator pair registry.

package translator

import "sync"

// Pair is a (from, to) format key, e.g. {"openai", "claude"}.
type Pair struct {
	From string
	To   string
}

// Translator transforms a request OR response body between two dialects.
type Translator func(body map[string]any) map[string]any

// Direction distinguishes request vs response translators (some pairs differ).
type Direction string

const (
	DirRequest  Direction = "request"
	DirResponse Direction = "response"
)

type key struct {
	Pair Pair
	Dir  Direction
}

var (
	regMu sync.RWMutex
	reg   = map[key]Translator{}
)

// Register adds a translator for a (pair, direction). Idempotent.
func Register(pair Pair, dir Direction, fn Translator) {
	if fn == nil {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	reg[key{Pair: pair, Dir: dir}] = fn
}

// Get returns the translator for (from→to, direction), or nil when none.
func Get(from, to string, dir Direction) Translator {
	regMu.RLock()
	defer regMu.RUnlock()
	return reg[key{Pair: Pair{From: from, To: to}, Dir: dir}]
}

// List returns every registered (from, to, direction) tuple.
func List() []struct {
	From, To  string
	Direction Direction
} {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]struct {
		From, To  string
		Direction Direction
	}, 0, len(reg))
	for k := range reg {
		out = append(out, struct {
			From, To  string
			Direction Direction
		}{k.Pair.From, k.Pair.To, k.Dir})
	}
	return out
}
