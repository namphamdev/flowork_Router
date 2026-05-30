// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Filter: find — pure path-list output; trim middle.
package filters

import (
	"strings"

	"github.com/flowork-os/flowork_Router/internal/rtk"
)

func init() { rtk.Register(&findFilter{}) }

type findFilter struct{}

func (f *findFilter) Name() string { return "find" }
func (f *findFilter) Detect(head string) bool {
	lines := strings.Split(head, "\n")
	nonEmpty := 0
	allPathLike := true
	for _, ln := range lines {
		s := strings.TrimSpace(ln)
		if s == "" {
			continue
		}
		nonEmpty++
		// path-like = no colon (rules out grep file:line:), starts with / or ./ or alnum.
		if strings.Contains(s, ":") {
			allPathLike = false
			break
		}
	}
	return nonEmpty >= 3 && allPathLike
}

func (f *findFilter) Apply(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= 60 {
		return text
	}
	first := lines[:30]
	last := lines[len(lines)-15:]
	cut := len(lines) - 45
	return strings.Join(first, "\n") +
		"\n…[" + itoa(cut) + " paths trimmed by RTK find]…\n" +
		strings.Join(last, "\n")
}
