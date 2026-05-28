// Request translator: OpenAI canonical → Cursor wire format.
// Cursor accepts OpenAI-compat shape directly; we strip the tools[] sentinel
// when empty and rename `messages` to `conversation` for legacy clients.
package request

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "openai", To: "cursor"}, translator.DirRequest, OpenAIToCursor)
}

// OpenAIToCursor returns a body Cursor backends can consume. Mostly identity.
func OpenAIToCursor(body map[string]any) map[string]any {
	out := make(map[string]any, len(body))
	for k, v := range body {
		out[k] = v
	}
	// Cursor strict mode rejects empty tools[]; drop it when nil/empty.
	if tools, ok := out["tools"].([]any); ok && len(tools) == 0 {
		delete(out, "tools")
	}
	return out
}
