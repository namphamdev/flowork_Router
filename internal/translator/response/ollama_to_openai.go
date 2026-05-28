// Response translator: Ollama /api/chat → OpenAI ChatCompletion.
package response

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "ollama", To: "openai"}, translator.DirResponse, OllamaToOpenAI)
}

// OllamaToOpenAI maps { message:{role,content}, done_reason } → OpenAI completion.
func OllamaToOpenAI(body map[string]any) map[string]any {
	var role, content, doneReason string
	if msg, ok := body["message"].(map[string]any); ok {
		role, _ = msg["role"].(string)
		content, _ = msg["content"].(string)
	}
	doneReason, _ = body["done_reason"].(string)
	finish := "stop"
	if doneReason == "length" {
		finish = "length"
	}
	if role == "" {
		role = "assistant"
	}
	return map[string]any{
		"id":     "chatcmpl-ollama",
		"object": "chat.completion",
		"model":  body["model"],
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": role, "content": content},
			"finish_reason": finish,
		}},
		"usage": map[string]any{
			"prompt_tokens":     int64Of(body["prompt_eval_count"]),
			"completion_tokens": int64Of(body["eval_count"]),
			"total_tokens":      int64Of(body["prompt_eval_count"]) + int64Of(body["eval_count"]),
		},
	}
}
