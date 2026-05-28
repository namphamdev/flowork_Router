// Helper: resolve per-model max_tokens with sensible defaults.
package helpers

import "strings"

// ModelMaxTokens maps a model id (or prefix) → maximum output tokens upstream
// will accept. Used by translators when the caller did not specify max_tokens.
var ModelMaxTokens = map[string]int{
	"claude-opus":       32_000,
	"claude-sonnet":     8192,
	"claude-haiku":      4096,
	"gpt-4o":            16_384,
	"gpt-4-turbo":       4096,
	"gpt-3.5-turbo":     4096,
	"gemini-1.5-pro":    8192,
	"gemini-2.5-pro":    65_536,
	"gemini-2.5-flash":  65_536,
	"gemini-3":          65_536,
	"deepseek-chat":     8192,
	"deepseek-reasoner": 8192,
	"qwen":              8192,
	"kimi":              8192,
}

// DefaultMaxTokens is the fallback when no rule matches.
const DefaultMaxTokens = 4096

// MaxTokensForModel returns the maximum output tokens to request for model.
// Returns the longest prefix match in ModelMaxTokens, or DefaultMaxTokens.
func MaxTokensForModel(model string) int {
	if model == "" {
		return DefaultMaxTokens
	}
	best := ""
	for prefix := range ModelMaxTokens {
		if strings.HasPrefix(model, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best != "" {
		return ModelMaxTokens[best]
	}
	return DefaultMaxTokens
}

// ResolveMaxTokens picks the explicit value when >0, otherwise the model's
// default ceiling. Used right before sending to upstream.
func ResolveMaxTokens(explicit int, model string) int {
	if explicit > 0 {
		return explicit
	}
	return MaxTokensForModel(model)
}
