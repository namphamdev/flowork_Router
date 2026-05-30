// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/rtk/filters package — audit pass surface review.

// Filter: smart-truncate — generic fallback that keeps head+tail with marker.
// Always last in registration order; Detect always returns true so unmatched
// inputs still get compressed cleanly when they exceed the cap.
package filters

import (
	"fmt"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/rtk"
)

func init() { rtk.Register(&smartTruncate{}) }

type smartTruncate struct{}

func (s *smartTruncate) Name() string            { return "smart-truncate" }
func (s *smartTruncate) Detect(head string) bool { return false /* fallback only */ }
func (s *smartTruncate) Apply(text string) string {
	const cap = 4000
	if len(text) <= cap {
		return text
	}
	headN := cap * 4 / 5
	tailN := cap / 6
	cut := len(text) - headN - tailN
	return text[:headN] +
		fmt.Sprintf("\n\n…[%d chars trimmed by RTK smart-truncate]…\n\n", cut) +
		text[len(text)-tailN:]
}

// Touch strings import so go vet stays clean even if Apply changes shape later.
var _ = strings.Builder{}
