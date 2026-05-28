// Request translator: OpenAI canonical → Kiro conversationState shape.
package request

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "openai", To: "kiro"}, translator.DirRequest, OpenAIToKiro)
}

// OpenAIToKiro splits OpenAI messages into Kiro's
// { conversationState: { history, currentMessage, modelId } } shape.
func OpenAIToKiro(body map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		if k == "messages" {
			continue
		}
		out[k] = v
	}
	var history []map[string]any
	var current map[string]any
	if msgs, ok := body["messages"].([]any); ok {
		for i, raw := range msgs {
			m, _ := raw.(map[string]any)
			if m == nil {
				continue
			}
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			entry := map[string]any{
				"role":    role,
				"content": []map[string]any{{"type": "text", "text": content}},
			}
			if i == len(msgs)-1 && role == "user" {
				current = entry
				continue
			}
			history = append(history, entry)
		}
	}
	out["conversationState"] = map[string]any{
		"chatTriggerType": "MANUAL",
		"history":         history,
		"currentMessage":  current,
		"modelId":         body["model"],
	}
	delete(out, "model")
	return out
}
