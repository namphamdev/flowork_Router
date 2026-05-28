// Response translator: Anthropic → OpenAI ChatCompletion.
package response

import (
	"github.com/flowork-os/flowork_Router/internal/translator"
)

func init() {
	translator.Register(translator.Pair{From: "claude", To: "openai"}, translator.DirResponse, ClaudeToOpenAI)
}

// ClaudeToOpenAI maps { id, content:[{type:"text",text}], stop_reason, usage }
// into OpenAI { id, object, choices:[{message:{role,content},finish_reason}], usage }.
func ClaudeToOpenAI(body map[string]any) map[string]any {
	id, _ := body["id"].(string)
	model, _ := body["model"].(string)
	stop, _ := body["stop_reason"].(string)
	finishReason := mapAnthropicStop(stop)

	var text string
	if blocks, ok := body["content"].([]any); ok {
		for _, b := range blocks {
			if m, ok := b.(map[string]any); ok {
				if m["type"] == "text" {
					if t, _ := m["text"].(string); t != "" {
						text += t
					}
				}
			}
		}
	}

	usageIn, _ := body["usage"].(map[string]any)
	usage := map[string]any{
		"prompt_tokens":     int64Of(usageIn["input_tokens"]),
		"completion_tokens": int64Of(usageIn["output_tokens"]),
	}
	usage["total_tokens"] = usage["prompt_tokens"].(int64) + usage["completion_tokens"].(int64)

	return map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"model":   model,
		"choices": []map[string]any{{
			"index":   0,
			"message": map[string]any{"role": "assistant", "content": text},
			"finish_reason": finishReason,
		}},
		"usage": usage,
	}
}

func mapAnthropicStop(s string) string {
	switch s {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	}
	if s == "" {
		return "stop"
	}
	return "stop"
}

func int64Of(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	}
	return 0
}
