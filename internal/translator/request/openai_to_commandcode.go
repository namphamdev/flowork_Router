// Request translator: OpenAI canonical → CommandCode (AI SDK v5 generate).
package request

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "openai", To: "commandcode"}, translator.DirRequest, OpenAIToCommandCode)
}

// OpenAIToCommandCode reshapes OpenAI messages into commandcode's
// /alpha/generate request: { model, prompt, history }. The last user message
// becomes prompt; everything before that becomes history.
func OpenAIToCommandCode(body map[string]any) map[string]any {
	out := map[string]any{
		"model": body["model"],
	}
	var prompt string
	var history []map[string]any
	if msgs, ok := body["messages"].([]any); ok {
		for i, raw := range msgs {
			m, _ := raw.(map[string]any)
			if m == nil {
				continue
			}
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			if i == len(msgs)-1 && role == "user" {
				prompt = content
				continue
			}
			history = append(history, map[string]any{"role": role, "content": content})
		}
	}
	out["prompt"] = prompt
	out["history"] = history
	return out
}
