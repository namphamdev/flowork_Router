// Filter: tree — `tree` command output; trim middle subtree.
package filters

import (
	"strings"

	"github.com/flowork-os/flowork_Router/internal/rtk"
)

func init() { rtk.Register(&treeFilter{}) }

type treeFilter struct{}

var reTreeGlyph = mustCompile(`[├└]──|│  `)

func (t *treeFilter) Name() string             { return "tree" }
func (t *treeFilter) Detect(head string) bool  { return reTreeGlyph.MatchString(head) }
func (t *treeFilter) Apply(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= 80 {
		return text
	}
	first := lines[:50]
	last := lines[len(lines)-20:]
	cut := len(lines) - 70
	return strings.Join(first, "\n") +
		"\n…[" + itoa(cut) + " tree entries trimmed by RTK]…\n" +
		strings.Join(last, "\n")
}
