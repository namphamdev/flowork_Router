// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/translator/request package — audit pass surface review.

// Request translator: OpenAI canonical → Ollama /api/chat shape.
// Ollama accepts OpenAI-compat as well; this is for /api/chat parity.
package request

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "openai", To: "ollama"}, translator.DirRequest, OpenAIToOllama)
}

// OpenAIToOllama drops `tools` (Ollama tool support is experimental) and
// flattens stream/temperature into `options`.
func OpenAIToOllama(body map[string]any) map[string]any {
	out := map[string]any{
		"model":    body["model"],
		"messages": body["messages"],
	}
	if v, ok := body["stream"]; ok {
		out["stream"] = v
	}
	opts := map[string]any{}
	if v, ok := body["temperature"]; ok {
		opts["temperature"] = v
	}
	if v, ok := body["top_p"]; ok {
		opts["top_p"] = v
	}
	if v, ok := body["max_tokens"]; ok {
		opts["num_predict"] = v
	}
	if len(opts) > 0 {
		out["options"] = opts
	}
	return out
}
