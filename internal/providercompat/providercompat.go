// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider adapter.

// provider compat prefix helpers.

package providercompat

import "strings"

const (
	openAIPrefix    = "openai-compatible-"
	anthropicPrefix = "anthropic-compatible-"
	responsesSuffix = "-responses"

	defaultOpenAIBase    = "https://api.openai.com/v1"
	defaultAnthropicBase = "https://api.anthropic.com/v1"
)

// IsOpenAICompatible reports whether the provider name designates an OpenAI-
// compatible backend via prefix.
func IsOpenAICompatible(provider string) bool {
	return strings.HasPrefix(provider, openAIPrefix)
}

// IsAnthropicCompatible reports whether the provider name designates an
// Anthropic-compatible backend via prefix.
func IsAnthropicCompatible(provider string) bool {
	return strings.HasPrefix(provider, anthropicPrefix)
}

// OpenAIAPIType returns "responses" when the provider name encodes the
// Responses API variant, "chat" otherwise (the default OpenAI chat-completions
// shape). Returns "" for providers that are not OpenAI-compatible.
func OpenAIAPIType(provider string) string {
	if !IsOpenAICompatible(provider) {
		return ""
	}
	if strings.Contains(provider, responsesSuffix) {
		return "responses"
	}
	return "chat"
}

// BuildOpenAICompatURL appends the right path suffix to a base URL based on
// the provider's API type. If baseURL is empty the canonical OpenAI base is
// used. Trailing slash on baseURL is normalised so the suffix never doubles.
func BuildOpenAICompatURL(baseURL, apiType string) string {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = defaultOpenAIBase
	}
	switch apiType {
	case "responses":
		return base + "/responses"
	default:
		return base + "/chat/completions"
	}
}

// BuildAnthropicCompatURL appends /messages to the base URL. Empty baseURL
// resolves to the canonical Anthropic API base.
func BuildAnthropicCompatURL(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = defaultAnthropicBase
	}
	return base + "/messages"
}

// ResolveFormat returns the canonical format string ("openai" / "openai-responses"
// / "anthropic" / "") given the provider name. Returns "" when the provider
// doesn't carry one of the prefixes — caller should fall back to the explicit
// format field in the provider record.
func ResolveFormat(provider string) string {
	if IsOpenAICompatible(provider) {
		if OpenAIAPIType(provider) == "responses" {
			return "openai-responses"
		}
		return "openai"
	}
	if IsAnthropicCompatible(provider) {
		return "anthropic"
	}
	return ""
}

// ResolveBaseURL returns the URL to call given a provider name + explicit
// baseURL. When the explicit baseURL is set we honour it (trimmed); when
// missing we fall back to vendor defaults. Returns "" when the provider
// type is unknown and no baseURL is supplied.
func ResolveBaseURL(provider, baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if base != "" {
		return base
	}
	switch {
	case IsOpenAICompatible(provider):
		return defaultOpenAIBase
	case IsAnthropicCompatible(provider):
		return defaultAnthropicBase
	}
	return ""
}
