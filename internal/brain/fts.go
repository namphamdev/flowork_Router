package brain

import (
	"fmt"
	"strings"
)

// ftsTable — the FTS5 virtual table name in a flowork Memory Palace DB.
// It stores content/wing/room directly, so retrieval needs no JOIN.
const ftsTable = "memory_fts"

// ftsTokens turns free-text into safe, quoted FTS5 tokens.
// Ported from flowork brain/fts.go: strip punctuation + FTS5 metacharacters,
// drop tokens shorter than 2 chars, quote each remaining token. Quoting keeps
// the query injection-safe and tolerant of noisy prompts. Empty slice means
// "no usable terms" — caller should skip the FTS lookup.
func ftsTokens(q string) []string {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	var parts []string
	for _, f := range strings.Fields(q) {
		var b strings.Builder
		for _, r := range f {
			switch r {
			case '"', '\'', '?', '.', ',', ':', ';', '!', '(', ')', '[', ']', '{', '}',
				'*', '/', '\\', '|', '&', '#', '@', '+', '=', '<', '>', '`', '~':
				continue
			default:
				b.WriteRune(r)
			}
		}
		clean := b.String()
		if len(clean) < 2 {
			continue
		}
		parts = append(parts, fmt.Sprintf(`"%s"`, clean))
	}
	return parts
}

// joinFTS joins quoted tokens with an operator ("AND" or "OR").
func joinFTS(tokens []string, op string) string {
	return strings.Join(tokens, " "+op+" ")
}
