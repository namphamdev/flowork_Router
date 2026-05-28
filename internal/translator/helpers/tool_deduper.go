// Tool deduplication: strip built-in or duplicate tool definitions when an
// equivalent MCP tool is already in the request. Reduces token bloat on
// Claude clients that mix Anthropic's built-in tools with MCP servers
// covering the same surface (web search, web fetch, Chrome control).
//
// Rule shape:
//
//	triggers: substring/regex patterns that must be present in the tool list
//	strip:    substring/regex patterns whose matching tools are removed
//
// All matching is case-sensitive and exact (no implicit prefix scan) —
// callers use the regex form for prefix rules.

package helpers

import "regexp"

// toolDedupPattern is either a literal name or a compiled regex; only one
// field is populated per entry.
type toolDedupPattern struct {
	literal string
	re      *regexp.Regexp
}

func litPattern(s string) toolDedupPattern { return toolDedupPattern{literal: s} }
func rePattern(p string) toolDedupPattern {
	return toolDedupPattern{re: regexp.MustCompile(p)}
}

type toolDedupRule struct {
	Triggers []toolDedupPattern
	Strip    []toolDedupPattern
}

// dedupRules holds the canonical rule set. Three current entries:
//   - Exa MCP → drop built-in WebSearch/WebFetch + workspace web_fetch
//   - Tavily MCP → drop the same trio
//   - Browser MCP → drop the Cowork Claude_in_Chrome connector
//
// Add a new entry to extend coverage; the matching is order-independent so
// rule order doesn't affect correctness.
var dedupRules = []toolDedupRule{
	{
		Triggers: []toolDedupPattern{
			litPattern("mcp__exa__web_search_exa"),
			litPattern("mcp__exa__web_fetch_exa"),
		},
		Strip: []toolDedupPattern{
			litPattern("WebSearch"),
			litPattern("WebFetch"),
			litPattern("mcp__workspace__web_fetch"),
		},
	},
	{
		Triggers: []toolDedupPattern{
			litPattern("mcp__tavily__tavily_search"),
			litPattern("mcp__tavily__tavily_extract"),
		},
		Strip: []toolDedupPattern{
			litPattern("WebSearch"),
			litPattern("WebFetch"),
			litPattern("mcp__workspace__web_fetch"),
		},
	},
	{
		Triggers: []toolDedupPattern{
			rePattern(`^mcp__browsermcp__`),
		},
		Strip: []toolDedupPattern{
			rePattern(`^mcp__Claude_in_Chrome__`),
		},
	},
}

// patternMatches reports whether name satisfies pat. Literal patterns
// require equality; regex patterns require a match anywhere.
func patternMatches(name string, pat toolDedupPattern) bool {
	if pat.re != nil {
		return pat.re.MatchString(name)
	}
	return pat.literal != "" && name == pat.literal
}

// extractToolName surfaces the tool id from either the OpenAI shape
// ({type:"function", function:{name}}) or the Anthropic shape ({name}).
func extractToolName(t any) string {
	tool, ok := t.(map[string]any)
	if !ok {
		return ""
	}
	if n, _ := tool["name"].(string); n != "" {
		return n
	}
	if fn, ok := tool["function"].(map[string]any); ok {
		if n, _ := fn["name"].(string); n != "" {
			return n
		}
	}
	return ""
}

// DedupeTools returns a copy of the tool list with built-in tools removed
// when an equivalent MCP tool is present, plus the slice of stripped tool
// names for logging.
func DedupeTools(tools []any) (out []any, stripped []string) {
	if len(tools) == 0 {
		return tools, nil
	}
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = extractToolName(t)
	}

	strip := map[string]bool{}
	for _, rule := range dedupRules {
		hasTrigger := false
		for _, n := range names {
			if n == "" {
				continue
			}
			for _, t := range rule.Triggers {
				if patternMatches(n, t) {
					hasTrigger = true
					break
				}
			}
			if hasTrigger {
				break
			}
		}
		if !hasTrigger {
			continue
		}
		for _, n := range names {
			if n == "" {
				continue
			}
			for _, s := range rule.Strip {
				if patternMatches(n, s) {
					strip[n] = true
					break
				}
			}
		}
	}

	if len(strip) == 0 {
		return tools, nil
	}
	out = make([]any, 0, len(tools))
	for i, t := range tools {
		if strip[names[i]] {
			continue
		}
		out = append(out, t)
	}
	stripped = make([]string, 0, len(strip))
	for n := range strip {
		stripped = append(stripped, n)
	}
	return out, stripped
}
