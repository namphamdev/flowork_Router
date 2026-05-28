// Reasoning Content Injector (DeepSeek / Kimi thinking).

package streamutil

import "strings"

// scope values mirror upstream constants.
const (
	scopeAll       = "all"
	scopeToolCalls = "toolCalls"
)

const reasoningPlaceholder = " "

// ProviderRules — provider name → injection scope. Lowercased keys.
var ProviderRules = map[string]string{
	"deepseek": scopeAll,
}

// ModelRule is a predicate-based fallback when the provider rule does not apply.
type ModelRule struct {
	Match func(model string) bool
	Scope string
}

// ModelRules mirror upstream's MODEL_RULES.
var ModelRules = []ModelRule{
	{Match: func(m string) bool { return strings.HasPrefix(m, "kimi-") }, Scope: scopeToolCalls},
	{Match: func(m string) bool { return strings.HasPrefix(m, "deepseek-") }, Scope: scopeAll},
}

// resolveScope returns the effective injection scope for (provider, model), or "" when none.
func resolveScope(provider, model string) string {
	if s, ok := ProviderRules[strings.ToLower(provider)]; ok {
		return s
	}
	for _, r := range ModelRules {
		if r.Match(model) {
			return r.Scope
		}
	}
	return ""
}

// shouldInject decides per-message whether the placeholder should be added.
func shouldInject(msg map[string]any, scope string) bool {
	if msg == nil || msg["role"] != "assistant" {
		return false
	}
	if rc, ok := msg["reasoning_content"].(string); ok && rc != "" {
		return false
	}
	if scope == scopeToolCalls {
		tc, _ := msg["tool_calls"].([]any)
		return len(tc) > 0
	}
	return true
}

// InjectReasoningContent rewrites body["messages"] in-place when the
// provider/model rule asks for it. Safe to call on any request body.
func InjectReasoningContent(provider, model string, body map[string]any) map[string]any {
	scope := resolveScope(provider, model)
	if scope == "" || body == nil {
		return body
	}
	msgs, ok := body["messages"].([]any)
	if !ok {
		return body
	}
	out := make([]any, len(msgs))
	for i, raw := range msgs {
		m, ok := raw.(map[string]any)
		if !ok {
			out[i] = raw
			continue
		}
		if shouldInject(m, scope) {
			clone := make(map[string]any, len(m)+1)
			for k, v := range m {
				clone[k] = v
			}
			clone["reasoning_content"] = reasoningPlaceholder
			out[i] = clone
		} else {
			out[i] = m
		}
	}
	body["messages"] = out
	return body
}
