// Request translator: OpenAI Responses API → OpenAI Chat Completions.
package request

import (
	"encoding/json"

	"github.com/flowork-os/flowork_Router/internal/translator"
	"github.com/flowork-os/flowork_Router/internal/translator/helpers"
)

func init() {
	translator.Register(translator.Pair{From: "openai-responses", To: "openai"}, translator.DirRequest, OpenAIResponsesToChat)
}

// OpenAIResponsesToChat moves `input` (string or array) into `messages` and
// promotes `instructions` to a system message.
func OpenAIResponsesToChat(body map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		if k == "input" || k == "instructions" || k == "max_output_tokens" {
			continue
		}
		out[k] = v
	}
	msgs := []map[string]any{}
	if instr, _ := body["instructions"].(string); instr != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": instr})
	}
	if raw, ok := body["input"]; ok {
		switch v := raw.(type) {
		case string:
			msgs = append(msgs, map[string]any{"role": "user", "content": v})
		default:
			rawJSON, _ := json.Marshal(v)
			parsed := helpers.ParseResponsesInput(rawJSON)
			msgs = append(msgs, parsed...)
		}
	}
	out["messages"] = msgs
	if mt, ok := body["max_output_tokens"]; ok {
		out["max_tokens"] = mt
	}
	return out
}
