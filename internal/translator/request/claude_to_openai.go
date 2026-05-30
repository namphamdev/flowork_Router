// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider request/response shape translator.

// Request translator: Anthropic Claude shape → OpenAI canonical.
package request

import (
	"github.com/flowork-os/flowork_Router/internal/translator"
	"github.com/flowork-os/flowork_Router/internal/translator/helpers"
)

func init() {
	translator.Register(translator.Pair{From: "claude", To: "openai"}, translator.DirRequest, ClaudeToOpenAI)
}

// ClaudeToOpenAI flattens Anthropic { system, messages: [{role, content:[{type:text,text}]}] }
// into OpenAI { messages: [{role:"system"|"user"|"assistant", content:string}] }.
func ClaudeToOpenAI(body map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		if k == "system" || k == "messages" {
			continue
		}
		out[k] = v
	}
	msgs := []map[string]any{}
	if sys := helpers.FlattenAnthropicSystem(body["system"]); sys != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": sys})
	}
	if arr, ok := body["messages"].([]any); ok {
		for _, raw := range arr {
			m, _ := raw.(map[string]any)
			if m == nil {
				continue
			}
			role, _ := m["role"].(string)
			content := helpers.FlattenAnthropicContent(m["content"])
			msgs = append(msgs, map[string]any{"role": role, "content": content})
		}
	}
	out["messages"] = msgs
	return out
}
