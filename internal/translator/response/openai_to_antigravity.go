// Response translator: OpenAI ChatCompletion → Antigravity (Gemini-shape) response.
// Antigravity returns Gemini wire; this just re-uses the openai-to-gemini mapping.
package response

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "openai", To: "antigravity"}, translator.DirResponse, OpenAIToAntigravity)
	translator.Register(translator.Pair{From: "openai", To: "gemini"}, translator.DirResponse, OpenAIToAntigravity)
}

// OpenAIToAntigravity emits { candidates:[{content,role:"model",parts:[…]},
// finishReason], usageMetadata } from an OpenAI completion.
func OpenAIToAntigravity(body map[string]any) map[string]any {
	model, _ := body["model"].(string)
	var text, finishReason string
	if ch, ok := body["choices"].([]any); ok && len(ch) > 0 {
		if c, ok := ch[0].(map[string]any); ok {
			if msg, ok := c["message"].(map[string]any); ok {
				text, _ = msg["content"].(string)
			}
			finishReason, _ = c["finish_reason"].(string)
		}
	}
	finish := "STOP"
	switch finishReason {
	case "length":
		finish = "MAX_TOKENS"
	case "content_filter":
		finish = "SAFETY"
	}
	usageIn, _ := body["usage"].(map[string]any)
	return map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"role":  "model",
					"parts": []map[string]any{{"text": text}},
				},
				"finishReason": finish,
				"index":        0,
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     int64Of(usageIn["prompt_tokens"]),
			"candidatesTokenCount": int64Of(usageIn["completion_tokens"]),
			"totalTokenCount":      int64Of(usageIn["total_tokens"]),
		},
		"modelVersion": model,
	}
}
