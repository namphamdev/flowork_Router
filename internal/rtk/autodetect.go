// RTK auto-detect orchestrator.

package rtk

import (
	"regexp"
	"strings"
)

// Detection-window heuristic patterns — extra signals on top of each filter's
// own Detect() so misclassification (build-output vs porcelain, grep vs
// dedup-log) is impossible regardless of registration order.
var (
	reGitDiffSig     = regexp.MustCompile(`(?m)^diff --git `)
	reGitDiffHunkSig = regexp.MustCompile(`(?m)^@@ `)
	reGitStatusSig   = regexp.MustCompile(`(?m)^On branch |^nothing to commit|^Changes (not |to be )|^Untracked files:`)
	rePorcelain      = regexp.MustCompile(`(?m)^[ MADRCU?!][ MADRCU?!] \S`)
	reBuildOutputSig = regexp.MustCompile(`(?im)^(npm (warn|error|ERR!)|yarn (warn|error)|\s*Compiling\s+\S+|\s*Downloading\s+\S+|added \d+ package|\[ERROR\]|BUILD (SUCCESS|FAILED)|\s*Finished\s+|Successfully (installed|built)|ERROR:)`)
	reTreeGlyph      = regexp.MustCompile("[├└]──|│  ")
	reLsRow          = regexp.MustCompile(`(?m)^[-dlbcps][rwx-]{9}`)
	reLsTotal        = regexp.MustCompile(`(?m)^total \d+$`)
	reSearchListHdr  = regexp.MustCompile(`(?m)^Files: `)
	reReadNumbered   = regexp.MustCompile(`(?m)^\s*\d+\|`)
)

// autoDetect picks the right filter using an explicit priority chain.
// Returns nil when nothing applies — caller falls back to head+tail trim.
func autoDetect(head string) Filter {
	// Snapshot the registry once to avoid holding the lock across loops.
	filtersMu.RLock()
	by := make(map[string]Filter, len(filters))
	for _, f := range filters {
		by[f.Name()] = f
	}
	filtersMu.RUnlock()

	// 1) git-diff: signatures are unambiguous.
	if reGitDiffSig.MatchString(head) || reGitDiffHunkSig.MatchString(head) {
		if f := by["git-diff"]; f != nil {
			return f
		}
	}

	// 2) git-status (the verbose form).
	if reGitStatusSig.MatchString(head) {
		if f := by["git-status"]; f != nil {
			return f
		}
	}

	// 3) build-output BEFORE porcelain — Cargo's "Compiling x" matches the
	//    short porcelain pattern, so build wins when both are present.
	if reBuildOutputSig.MatchString(head) {
		if f := by["build-output"]; f != nil {
			return f
		}
	}

	// 4) porcelain (short git status).
	if isMostlyPorcelain(head) {
		if f := by["git-status"]; f != nil {
			return f
		}
	}

	// 5) grep: first 5 non-empty lines, ANY matches `path:lineno:content`.
	lines := strings.Split(head, "\n")
	nonEmpty := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			nonEmpty = append(nonEmpty, ln)
		}
	}
	first5 := nonEmpty
	if len(first5) > 5 {
		first5 = first5[:5]
	}
	for _, ln := range first5 {
		if isGrepLine(ln) {
			if f := by["grep"]; f != nil {
				return f
			}
			break
		}
	}

	// 6) find: ALL non-empty lines path-like, ≥ 3 lines.
	if len(nonEmpty) >= 3 {
		allPath := true
		for _, ln := range nonEmpty {
			if !isPathLike(ln) {
				allPath = false
				break
			}
		}
		if allPath {
			if f := by["find"]; f != nil {
				return f
			}
		}
	}

	// 7) tree.
	if reTreeGlyph.MatchString(head) {
		if f := by["tree"]; f != nil {
			return f
		}
	}

	// 8) ls -la: header `total N` OR ≥ 3 rows starting with perms string.
	if reLsTotal.MatchString(head) || countRegexpMatches(reLsRow, head) >= 3 {
		if f := by["ls"]; f != nil {
			return f
		}
	}

	// 9) search-list (Cursor-style glob header).
	if reSearchListHdr.MatchString(head) {
		if f := by["search-list"]; f != nil {
			return f
		}
	}

	// 10) read-numbered: many "  N|content" lines.
	if countRegexpMatches(reReadNumbered, head) >= 10 {
		if f := by["read-numbered"]; f != nil {
			return f
		}
	}

	// 11) dedup-log: fallback for any multi-line blob with ≥ 5 non-empty lines.
	if len(nonEmpty) >= 5 {
		if f := by["dedup-log"]; f != nil {
			return f
		}
	}

	// 12) smart-truncate: last resort for huge unstructured blobs.
	if strings.Count(head, "\n") >= 40 {
		if f := by["smart-truncate"]; f != nil {
			return f
		}
	}
	return nil
}

func isGrepLine(line string) bool {
	first := strings.IndexByte(line, ':')
	if first < 0 {
		return false
	}
	second := strings.IndexByte(line[first+1:], ':')
	if second < 0 {
		return false
	}
	num := line[first+1 : first+1+second]
	if num == "" {
		return false
	}
	for _, c := range num {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isPathLike(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return false
	}
	if strings.ContainsRune(t, ':') {
		return false
	}
	return strings.HasPrefix(t, ".") || strings.HasPrefix(t, "/") || strings.ContainsRune(t, '/')
}

func isMostlyPorcelain(head string) bool {
	lines := strings.Split(head, "\n")
	nonEmpty := 0
	matches := 0
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		nonEmpty++
		if rePorcelain.MatchString(ln) {
			matches++
		}
	}
	if nonEmpty < 3 {
		return false
	}
	return matches*100/nonEmpty >= 60 // 60% threshold (mirrors upstream)
}

func countRegexpMatches(re *regexp.Regexp, s string) int {
	return len(re.FindAllString(s, -1))
}
