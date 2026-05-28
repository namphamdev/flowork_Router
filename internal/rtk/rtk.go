// RTK Token Saver (Advanced: tool-output auto-detect).

package rtk

import (
	"fmt"
	"regexp"
	"sync"
)

// Filter is a single tool-output compactor. Detect returns true when the input
// matches this filter's signature; Apply returns the compressed text.
type Filter interface {
	Name() string
	Detect(head string) bool
	Apply(text string) string
}

var (
	filtersMu sync.RWMutex
	filters   []Filter
)

// Register adds a filter to the registry. Called from init() in each filter
// file so the registry is populated before first use.
func Register(f Filter) {
	filtersMu.Lock()
	defer filtersMu.Unlock()
	filters = append(filters, f)
}

// detectWindow caps how much of the input we scan to choose a filter. Mirrors
// upstream's autodetect (Rust port). 8 KB is plenty for any heuristic signal.
const detectWindow = 8 * 1024

// Compress detects the right filter and applies it. Falls back to a head+tail
// trim when no filter matches. Returns (compressed, savedBytes).
func Compress(text string, cap int) (string, int) {
	if cap <= 0 || len(text) <= cap {
		return text, 0
	}
	head := text
	if len(head) > detectWindow {
		head = head[:detectWindow]
	}
	// autoDetect handles its own registry snapshot + priority chain. The old
	// "first-Register-wins" pickFilter is kept below for backwards compat
	// callers but is no longer the default path — autodetect is strictly
	// better (explicit ordering + smart heuristics for grep/find/porcelain).
	pick := autoDetect(head)
	var out string
	if pick != nil {
		out = pick.Apply(text)
	} else {
		out = fallbackHeadTail(text, cap)
	}
	if len(out) < len(text) {
		return out, len(text) - len(out)
	}
	return text, 0
}

// pickFilter — legacy first-Register-wins detection. Kept for tests that
// want to assert which filter would match without going through autoDetect's
// priority chain. New callers should use autoDetect instead.
func pickFilter(head string) Filter {
	filtersMu.RLock()
	defer filtersMu.RUnlock()
	for _, f := range filters {
		if f.Detect(head) {
			return f
		}
	}
	return nil
}

// fallbackHeadTail keeps the head (cap*0.8) + tail (cap*0.15) with a marker.
func fallbackHeadTail(s string, cap int) string {
	if len(s) <= cap {
		return s
	}
	headN := cap * 4 / 5
	tailN := cap / 6
	if headN+tailN >= len(s) {
		return s
	}
	cut := len(s) - headN - tailN
	return s[:headN] +
		fmt.Sprintf("\n\n…[%d chars trimmed by RTK]…\n\n", cut) +
		s[len(s)-tailN:]
}

// Compiled regexp helper — every filter compiles its own patterns at init.
func mustCompile(p string) *regexp.Regexp { return regexp.MustCompile(p) }
