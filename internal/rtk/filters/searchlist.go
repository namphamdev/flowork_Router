// Filter: search-list — Cursor IDE Glob search header output.
package filters

import (
	"strings"

	"github.com/flowork-os/flowork_Router/internal/rtk"
)

func init() { rtk.Register(&searchList{}) }

type searchList struct{}

var reSearchListHeader = mustCompile(`(?m)^(?:Glob|Search|Files matching) `)

func (s *searchList) Name() string             { return "search-list" }
func (s *searchList) Detect(head string) bool  { return reSearchListHeader.MatchString(head) }
func (s *searchList) Apply(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= 50 {
		return text
	}
	first := lines[:30]
	last := lines[len(lines)-10:]
	cut := len(lines) - 40
	return strings.Join(first, "\n") +
		"\n…[" + itoa(cut) + " matches trimmed by RTK search-list]…\n" +
		strings.Join(last, "\n")
}
