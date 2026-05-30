// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — RTK (Router Tool Kit) filter.

// Filter: build-output — keep ERROR/FAILED lines + last N progress lines.
package filters

import (
	"strings"

	"github.com/flowork-os/flowork_Router/internal/rtk"
)

func init() { rtk.Register(&buildOutput{}) }

type buildOutput struct{}

var reBuildOutput = mustCompile(`(?im)^(npm (warn|error|ERR!)|yarn (warn|error)|\s*Compiling\s+\S+|\s*Downloading\s+\S+|added \d+ package|\[ERROR\]|BUILD (SUCCESS|FAILED)|\s*Finished\s+|Successfully (installed|built)|ERROR:|FAILED:|Traceback|panic:)`)

func (b *buildOutput) Name() string            { return "build-output" }
func (b *buildOutput) Detect(head string) bool { return reBuildOutput.MatchString(head) }
func (b *buildOutput) Apply(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= 60 {
		return text
	}
	// Capture lines that look critical (errors/panics/tracebacks) — these we
	// always preserve, but cap so a build full of "WARN" lines does not blow
	// up to its original size.
	critical := mustCompile(`(?i)\b(error|failed|traceback|panic|fatal)\b`)
	const maxCritical = 40
	criticalLines := make([]string, 0, maxCritical)
	dropped := 0
	for _, ln := range lines {
		if critical.MatchString(ln) {
			if len(criticalLines) < maxCritical {
				criticalLines = append(criticalLines, ln)
			} else {
				dropped++
			}
		}
	}
	// Always keep the first 5 (banner) + last 20 (final state).
	headN := 5
	tailN := 20
	if len(lines) < headN+tailN+10 {
		return text
	}
	head := lines[:headN]
	tail := lines[len(lines)-tailN:]
	totalKeptByHeadTail := headN + tailN
	totalCutBy := len(lines) - totalKeptByHeadTail - len(criticalLines)
	if totalCutBy < 0 {
		totalCutBy = 0
	}

	out := make([]string, 0, headN+len(criticalLines)+tailN+3)
	out = append(out, head...)
	if len(criticalLines) > 0 {
		out = append(out, "…["+itoa(totalCutBy)+" progress lines trimmed by RTK build-output]…")
		out = append(out, criticalLines...)
		if dropped > 0 {
			out = append(out, "…[+"+itoa(dropped)+" more critical lines]…")
		}
	} else {
		out = append(out, "…["+itoa(totalCutBy)+" progress lines trimmed by RTK build-output]…")
	}
	out = append(out, tail...)
	return strings.Join(out, "\n")
}
