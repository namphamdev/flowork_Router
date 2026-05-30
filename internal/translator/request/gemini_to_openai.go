// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider request/response translator.

// Request translator: Gemini shape → OpenAI canonical.
package request

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "gemini", To: "openai"}, translator.DirRequest, GeminiToOpenAI)
}

// GeminiToOpenAI converts { contents:[{role,parts:[{text}]}], systemInstruction }
// into { messages:[{role,content}] }.
func GeminiToOpenAI(body map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		if k == "contents" || k == "systemInstruction" {
			continue
		}
		out[k] = v
	}
	msgs := []map[string]any{}
	if sys, ok := body["systemInstruction"].(map[string]any); ok {
		if txt := joinParts(sys["parts"]); txt != "" {
			msgs = append(msgs, map[string]any{"role": "system", "content": txt})
		}
	}
	if contents, ok := body["contents"].([]any); ok {
		for _, raw := range contents {
			c, _ := raw.(map[string]any)
			if c == nil {
				continue
			}
			role, _ := c["role"].(string)
			if role == "model" {
				role = "assistant"
			}
			msgs = append(msgs, map[string]any{"role": role, "content": joinParts(c["parts"])})
		}
	}
	out["messages"] = msgs
	return out
}

func joinParts(parts any) string {
	arr, ok := parts.([]any)
	if !ok {
		return ""
	}
	out := ""
	for _, p := range arr {
		if m, ok := p.(map[string]any); ok {
			if t, _ := m["text"].(string); t != "" {
				out += t
			}
		}
	}
	return out
}
