// Request translator: OpenAI canonical → Vertex AI generateContent shape.
package request

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "openai", To: "vertex"}, translator.DirRequest, OpenAIToVertex)
}

// OpenAIToVertex emits Vertex's { contents:[{role,parts:[{text}]}],
// systemInstruction, generationConfig } shape. Tools are passed through.
func OpenAIToVertex(body map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		if k == "messages" || k == "max_tokens" || k == "temperature" || k == "top_p" || k == "system" {
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
	gc := map[string]any{}
	if v, ok := body["max_tokens"]; ok {
		gc["maxOutputTokens"] = v
	}
	if v, ok := body["temperature"]; ok {
		gc["temperature"] = v
	}
	if v, ok := body["top_p"]; ok {
		gc["topP"] = v
	}
	if len(gc) > 0 {
		out["generationConfig"] = gc
	}
	return out
}
