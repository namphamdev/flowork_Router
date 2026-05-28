// Cost-tier classifier (heuristic, no external dep).

package router

import (
	"strings"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// CostTier is the classification result. Providers are filtered by their
// tier:<value> tag in Data["tags"].
type CostTier string

const (
	TierCheap    CostTier = "cheap"
	TierStandard CostTier = "standard"
	TierStrong   CostTier = "strong"
)

// ClassifyCost returns the tier for req under the given settings. Rules
// (first match wins, from strongest to cheapest):
//  1. tool_use present + StrongOnToolUse → strong
//  2. code block (``` fenced) detected + StrongOnCode → strong
//  3. multi-turn ≥ StrongMinMessages → strong
//  4. total input chars > StandardMaxChars → strong
//  5. total input chars > CheapMaxChars   → standard
//  6. otherwise → cheap
func ClassifyCost(req OpenAIRequest, cfg store.CostRouting) CostTier {
	// Tool calling — almost always implies agentic loop, big reasoning.
	if cfg.StrongOnToolUse && requestHasToolUse(req) {
		return TierStrong
	}

	totalChars := 0
	hasCode := false
	for _, m := range req.Messages {
		totalChars += len(m.Content)
		if !hasCode && cfg.StrongOnCode && containsCodeBlock(m.Content) {
			hasCode = true
		}
	}
	if hasCode {
		return TierStrong
	}

	if cfg.StrongMinMessages > 0 && len(req.Messages) >= cfg.StrongMinMessages {
		return TierStrong
	}

	if cfg.StandardMaxChars > 0 && totalChars > cfg.StandardMaxChars {
		return TierStrong
	}
	if cfg.CheapMaxChars > 0 && totalChars > cfg.CheapMaxChars {
		return TierStandard
	}
	return TierCheap
}

// requestHasToolUse reports whether the request carries tool definitions or
// any assistant message has tool_calls populated.
func requestHasToolUse(req OpenAIRequest) bool {
	if len(req.Tools) > 2 { // more than `[]` or `{}`
		return true
	}
	for _, m := range req.Messages {
		if len(m.ToolCalls) > 2 {
			return true
		}
		if m.ToolCallID != "" {
			return true
		}
	}
	return false
}

// containsCodeBlock detects triple-backtick fenced blocks. Cheap substring
// scan — false positives on raw "```" in prose are acceptable (errs strong).
func containsCodeBlock(s string) bool {
	return strings.Contains(s, "```")
}

// filterByTier keeps providers carrying the tier:<t> tag. Returns the input
// unchanged when nothing matches (so cost-routing never starves the request).
func filterByTier(matches []store.ProviderConnection, tier CostTier) []store.ProviderConnection {
	tag := "tier:" + string(tier)
	var out []store.ProviderConnection
	for _, p := range matches {
		if providerHasTag(p, tag) {
			out = append(out, p)
		}
	}
	return out
}

// hasActiveProviderForModel reports whether at least one of matches looks
// like the user's explicit choice (a provider serving exactly req.Model).
// When HonorExplicitModel is on, this short-circuits tier filtering so a
// client that names "claude-sonnet-4" still gets sonnet, not haiku.
func hasActiveProviderForModel(matches []store.ProviderConnection, model string) bool {
	if model == "" {
		return false
	}
	for _, p := range matches {
		models, _ := p.Data[store.CfgModels].([]any)
		for _, m := range models {
			if s, ok := m.(string); ok && s == model {
				return true
			}
		}
	}
	return false
}
