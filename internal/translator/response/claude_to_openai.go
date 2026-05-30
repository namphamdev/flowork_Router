// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider request/response translator.

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
	usage := buildOpenAIUsageFromAnthropic(usageIn)

	return map[string]any{
		"id":     id,
		"object": "chat.completion",
		"model":  model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": text},
			"finish_reason": finishReason,
		}},
		"usage": usage,
	}
}

// buildOpenAIUsageFromAnthropic translates an Anthropic usage block into the
// OpenAI shape, preserving prompt-cache breakdowns:
//
//	prompt_tokens     = input_tokens + cache_read_input_tokens + cache_creation_input_tokens
//	completion_tokens = output_tokens
//	total_tokens      = prompt_tokens + completion_tokens
//
// When cache_read or cache_creation are non-zero, they are exposed under
//
//	prompt_tokens_details.cached_tokens          (read hits)
//	prompt_tokens_details.cache_creation_tokens  (writes)
//
// matching the breakdown OpenAI uses for its own cache reporting. The raw
// Anthropic fields are also surfaced verbatim so observability tooling that
// reads "input_tokens" / "output_tokens" keeps working.
func buildOpenAIUsageFromAnthropic(usageIn map[string]any) map[string]any {
	input := int64Of(usageIn["input_tokens"])
	output := int64Of(usageIn["output_tokens"])
	cacheRead := int64Of(usageIn["cache_read_input_tokens"])
	cacheCreate := int64Of(usageIn["cache_creation_input_tokens"])

	prompt := input + cacheRead + cacheCreate
	usage := map[string]any{
		"prompt_tokens":     prompt,
		"completion_tokens": output,
		"total_tokens":      prompt + output,
		// Anthropic-native passthrough for callers that prefer the original
		// shape (e.g. log filters that already key on these names).
		"input_tokens":  input,
		"output_tokens": output,
	}
	if cacheRead > 0 || cacheCreate > 0 {
		details := map[string]any{}
		if cacheRead > 0 {
			details["cached_tokens"] = cacheRead
			usage["cache_read_input_tokens"] = cacheRead
		}
		if cacheCreate > 0 {
			details["cache_creation_tokens"] = cacheCreate
			usage["cache_creation_input_tokens"] = cacheCreate
		}
		usage["prompt_tokens_details"] = details
	}
	return usage
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
