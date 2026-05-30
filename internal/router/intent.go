// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Router dispatch/orchestrator.

// Per-Intent Multiplexing (private → local).

package router

import (
	"strings"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// promptIsPrivate reports whether any pattern (case-insensitive substring)
// occurs in the request's user/system text.
func promptIsPrivate(req OpenAIRequest, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	var sb strings.Builder
	for _, m := range req.Messages {
		if m.Role == "user" || m.Role == "system" {
			sb.WriteString(strings.ToLower(m.Content))
			sb.WriteByte('\n')
		}
	}
	text := sb.String()
	for _, p := range patterns {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" && strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// filterByTag keeps providers whose Data["tags"] contains tag (case-insensitive).
func filterByTag(matches []store.ProviderConnection, tag string) []store.ProviderConnection {
	tag = strings.ToLower(strings.TrimSpace(tag))
	var out []store.ProviderConnection
	for _, p := range matches {
		if providerHasTag(p, tag) {
			out = append(out, p)
		}
	}
	return out
}

func providerHasTag(p store.ProviderConnection, tag string) bool {
	tags, _ := p.Data["tags"].([]any)
	for _, t := range tags {
		if s, ok := t.(string); ok && strings.ToLower(s) == tag {
			return true
		}
	}
	return false
}
