// reasoning_content injector + DeepSeek v4-Pro alias rewriting.
//
// Some thinking-mode providers (DeepSeek family, Kimi) reject assistant
// messages that are missing a non-empty reasoning_content field. Clients
// in OpenAI format don't carry it, so we inject a single-space placeholder
// before the request leaves this process.
//
// DeepSeek's v4-Pro model ships with "-max" / "-none" aliases that tune
// thinking-mode + reasoning_effort. We rewrite them to the base model id
// + the matching extra_body knobs upstream expects.

package helpers

import "strings"

const reasoningPlaceholder = " "

// reasoningScope identifies which assistant messages need the placeholder:
//
//	scopeAll       — every assistant turn
//	scopeToolCalls — only assistant turns that carry tool_calls
//	scopeNone      — never (no-op)
type reasoningScope int

const (
	scopeNone reasoningScope = iota
	scopeAll
	scopeToolCalls
)

// pickReasoningScope returns the scope for (provider, model). Provider
// rules win over model rules; unknown combinations get scopeNone.
func pickReasoningScope(provider, model string) reasoningScope {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.ToLower(strings.TrimSpace(model))

	// Provider-level rules.
	if provider == "deepseek" {
		return scopeAll
	}
	// Model-prefix rules (case-insensitive). Order matters when prefixes overlap.
	switch {
	case strings.HasPrefix(model, "deepseek-"):
		return scopeAll
	case strings.HasPrefix(model, "kimi-"):
		return scopeToolCalls
	}
	return scopeNone
}

// InjectReasoningContent walks body.messages and inserts a non-empty
// reasoning_content placeholder on assistant messages that match the
// provider/model rule. Operates on the raw decoded shape so any caller
// holding map[string]any can use it; mutates the messages slice in place.
func InjectReasoningContent(provider, model string, body map[string]any) {
	scope := pickReasoningScope(provider, model)
	if scope == scopeNone {
		return
	}
	msgs, ok := body["messages"].([]any)
	if !ok {
		return
	}
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role != "assistant" {
			continue
		}
		if rc, _ := msg["reasoning_content"].(string); rc != "" {
			continue // caller already supplied one
		}
		if scope == scopeToolCalls {
			tcs, _ := msg["tool_calls"].([]any)
			if len(tcs) == 0 {
				continue
			}
		}
		msg["reasoning_content"] = reasoningPlaceholder
	}
}

// deepseekV4ProAliases maps the synthetic "-max" / "-none" model ids to the
// base id + thinking knobs upstream accepts. Returns (newModel, alias, ok).
type deepseekAlias struct {
	ThinkingType    string // "enabled" or "disabled"
	ReasoningEffort string // "max" or "" (strip)
}

var deepseekV4ProAliases = map[string]deepseekAlias{
	"deepseek-v4-pro-max":  {ThinkingType: "enabled", ReasoningEffort: "max"},
	"deepseek-v4-pro-none": {ThinkingType: "disabled", ReasoningEffort: ""},
}

// ApplyDeepSeekV4ProAlias rewrites body.model + body.extra_body.thinking +
// body.reasoning_effort when (provider, model) matches a known v4-Pro alias.
// Returns the (possibly unchanged) body — same object, mutated in place when
// rewriting fires.
func ApplyDeepSeekV4ProAlias(provider string, body map[string]any) {
	if strings.ToLower(provider) != "deepseek" {
		return
	}
	model, _ := body["model"].(string)
	alias, ok := deepseekV4ProAliases[strings.ToLower(model)]
	if !ok {
		return
	}
	body["model"] = "deepseek-v4-pro"
	extra, _ := body["extra_body"].(map[string]any)
	if extra == nil {
		extra = map[string]any{}
		body["extra_body"] = extra
	}
	thinking, _ := extra["thinking"].(map[string]any)
	if thinking == nil {
		thinking = map[string]any{}
		extra["thinking"] = thinking
	}
	thinking["type"] = alias.ThinkingType
	if alias.ReasoningEffort != "" {
		body["reasoning_effort"] = alias.ReasoningEffort
	} else {
		delete(body, "reasoning_effort")
	}
}
