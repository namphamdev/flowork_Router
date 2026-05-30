// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Filter: git-diff — keep file headers + hunks; drop pure context noise.
package filters

import (
	"strings"

	"github.com/flowork-os/flowork_Router/internal/rtk"
)

func init() { rtk.Register(&gitDiff{}) }

type gitDiff struct{}

var (
	reGitDiff      = mustCompile(`(?m)^diff --git `)
	reGitDiffHunk  = mustCompile(`(?m)^@@ `)
	reGitDiffStart = mustCompile(`(?m)^(diff --git|index |@@ |\+\+\+ |--- )`)
)

func (g *gitDiff) Name() string { return "git-diff" }
func (g *gitDiff) Detect(head string) bool {
	return reGitDiff.MatchString(head) || reGitDiffHunk.MatchString(head)
}
func (g *gitDiff) Apply(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	dropped := 0
	for _, ln := range lines {
		// Keep headers, hunks, +/- lines. Drop pure unchanged context lines
		// (those start with a single space).
		if strings.HasPrefix(ln, " ") && !strings.HasPrefix(ln, "  ") {
			dropped++
			continue
		}
		out = append(out, ln)
	}
	res := strings.Join(out, "\n")
	if dropped > 0 {
		res += "\n\n…[" + itoa(dropped) + " context lines trimmed by RTK git-diff]…"
	}
	return res
}
