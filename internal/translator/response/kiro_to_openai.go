// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/translator/response package — audit pass surface review.

// Response translator: Kiro CodeWhisperer → OpenAI ChatCompletion.
package response

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "kiro", To: "openai"}, translator.DirResponse, KiroToOpenAI)
}

// KiroToOpenAI extracts assistantResponse.text and wraps it in an OpenAI completion.
// Kiro shape (post-decode): { content: { text: "…" }, eventId, … }
func KiroToOpenAI(body map[string]any) map[string]any {
	var text string
	if content, ok := body["content"].(map[string]any); ok {
		text, _ = content["text"].(string)
	}
	if text == "" {
		text, _ = body["text"].(string)
	}
	return map[string]any{
		"id":     "chatcmpl-kiro",
		"object": "chat.completion",
		"model":  body["modelId"],
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": text},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     int64(0),
			"completion_tokens": int64(0),
			"total_tokens":      int64(0),
		},
	}
}
