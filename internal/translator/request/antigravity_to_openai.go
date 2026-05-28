// Request translator: Antigravity wire (already-wrapped Cloud Code Assist) → OpenAI.
// Unwraps { project, model, request: <body> } and feeds the inner body into
// the gemini→openai translator since the wire is Gemini-shape.
package request

import "github.com/flowork-os/flowork_Router/internal/translator"

func init() {
	translator.Register(translator.Pair{From: "antigravity", To: "openai"}, translator.DirRequest, AntigravityToOpenAI)
}

// AntigravityToOpenAI unwraps the Cloud Code Assist wrapper and delegates.
func AntigravityToOpenAI(body map[string]any) map[string]any {
	inner, ok := body["request"].(map[string]any)
	if !ok {
		// Not wrapped — assume already Gemini shape.
		return GeminiToOpenAI(body)
	}
	out := GeminiToOpenAI(inner)
	if m, ok := body["model"].(string); ok && m != "" {
		out["model"] = m
	}
	return out
}
