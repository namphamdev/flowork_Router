// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/translator/response package — audit pass surface review.

// Response translator: Cursor wire → OpenAI canonical.
// Cursor returns OpenAI-compat; mostly identity (strip the extras).
package response

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "cursor", To: "openai"}, translator.DirResponse, CursorToOpenAI)
}

// CursorToOpenAI passes through the response after dropping Cursor-specific
// telemetry fields (`cursor_metadata`, `analytics`).
func CursorToOpenAI(body map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		if k == "cursor_metadata" || k == "analytics" {
			continue
		}
		out[k] = v
	}
	return out
}
