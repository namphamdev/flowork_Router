// Request translator: OpenAI canonical → Gemini shape.
package request

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "openai", To: "gemini"}, translator.DirRequest, OpenAIToGemini)
}

// OpenAIToGemini converts { messages:[…] } into Gemini
// { systemInstruction, contents:[{role,parts:[{text}]}] }.
func OpenAIToGemini(body map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		if k == "messages" || k == "system" {
			continue
		}
		out[k] = v
	}
	contents := []map[string]any{}
	var systemText string
	if msgs, ok := body["messages"].([]any); ok {
		for _, raw := range msgs {
			m, _ := raw.(map[string]any)
			if m == nil {
				continue
			}
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			if role == "system" {
				if systemText == "" {
					systemText = content
				} else {
					systemText += "\n\n" + content
				}
				continue
			}
			if role == "assistant" {
				role = "model"
			}
			contents = append(contents, map[string]any{
				"role":  role,
				"parts": []map[string]any{{"text": content}},
			})
		}
	}
	if systemText != "" {
		out["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": systemText}},
		}
	}
	out["contents"] = contents
	return out
}
