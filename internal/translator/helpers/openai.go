// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider request/response translator.

// Helper: OpenAI shape ↔ canonical.
package helpers

// OpenAIFinishReason classifies the upstream into the OpenAI vocabulary.
func OpenAIFinishReason(reason string) string {
	switch reason {
	case "stop", "length", "tool_calls", "content_filter":
		return reason
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	}
	if reason == "" {
		return "stop"
	}
	return reason
}

// MergeSystemMessages joins multiple system messages into one (OpenAI accepts
// only a single system message at the head). Returns "" when none.
func MergeSystemMessages(msgs []map[string]any) (string, []map[string]any) {
	var systemParts []string
	rest := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		role, _ := m["role"].(string)
		if role == "system" {
			if c, _ := m["content"].(string); c != "" {
				systemParts = append(systemParts, c)
			}
			continue
		}
		rest = append(rest, m)
	}
	merged := ""
	for i, p := range systemParts {
		if i > 0 {
			merged += "\n\n"
		}
		merged += p
	}
	return merged, rest
}

// EnsureLastUserMessage promotes the last user message to the end of msgs (or
// appends an empty user when none) — required by some thinking-mode providers.
func EnsureLastUserMessage(msgs []map[string]any) []map[string]any {
	if len(msgs) == 0 {
		return []map[string]any{{"role": "user", "content": ""}}
	}
	if r, _ := msgs[len(msgs)-1]["role"].(string); r == "user" {
		return msgs
	}
	return append(msgs, map[string]any{"role": "user", "content": ""})
}
