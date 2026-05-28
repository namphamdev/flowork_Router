// Helper: OpenAI Responses API shape ↔ canonical.
package helpers

import "encoding/json"

// ParseResponsesInput accepts the `input` field of a /v1/responses request,
// which can be a plain string or a list of `{role, content}` items. Returns
// canonical chat messages.
func ParseResponsesInput(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	// Plain string → single user message
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return []map[string]any{{"role": "user", "content": s}}
	}
	// Array of items
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return []map[string]any{{"role": "user", "content": string(raw)}}
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		// Try string-in-array
		var s string
		if err := json.Unmarshal(item, &s); err == nil {
			out = append(out, map[string]any{"role": "user", "content": s})
			continue
		}
		var obj struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(item, &obj); err == nil {
			role := obj.Role
			if role == "" {
				role = "user"
			}
			out = append(out, map[string]any{"role": role, "content": FlattenAnthropicContent(jsonToAny(obj.Content))})
		}
	}
	return out
}

// EncodeResponsesOutput wraps a plain text reply in the Responses API
// "output[]" shape that the OpenAI Responses dialect expects.
func EncodeResponsesOutput(text string) []map[string]any {
	return []map[string]any{
		{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": text},
			},
		},
	}
}

func jsonToAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	_ = json.Unmarshal(raw, &v)
	return v
}
