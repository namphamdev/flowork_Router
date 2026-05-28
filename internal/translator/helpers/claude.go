// Helper: Claude/Anthropic shape ↔ canonical.
package helpers

import "strings"

// FlattenAnthropicSystem accepts either a string or `[{type:"text", text:...}]`
// system field and returns the flattened string.
func FlattenAnthropicSystem(system any) string {
	switch v := system.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "text" {
					if txt, _ := m["text"].(string); txt != "" {
						parts = append(parts, txt)
					}
				}
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

// FlattenAnthropicContent accepts either a string or `[{type:"text", text:...}]`
// content field and returns the joined string (no separator — matches upstream).
func FlattenAnthropicContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "text" {
					if txt, _ := m["text"].(string); txt != "" {
						b.WriteString(txt)
					}
				}
			}
		}
		return b.String()
	}
	return ""
}

// MapClaudeStopReason maps OpenAI finish_reason → Anthropic stop_reason.
func MapClaudeStopReason(finishReason string) string {
	switch finishReason {
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "stop":
		return "end_turn"
	}
	if finishReason == "" {
		return "end_turn"
	}
	return finishReason
}
