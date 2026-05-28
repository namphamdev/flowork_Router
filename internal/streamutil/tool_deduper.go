// Tool Deduper (strip duplicates when MCP equivalents).

package streamutil

import "regexp"

// Pattern is either an exact string match or a regex.
type Pattern struct {
	Exact string
	Regex *regexp.Regexp
}

// DedupeRule says: when ALL Triggers are present in the tool set, drop every
// tool matching any pattern in Strip.
type DedupeRule struct {
	Triggers []Pattern
	Strip    []Pattern
}

func exact(s string) Pattern { return Pattern{Exact: s} }
func re(p string) Pattern    { return Pattern{Regex: regexp.MustCompile(p)} }
func match(name string, p Pattern) bool {
	if p.Exact != "" {
		return name == p.Exact
	}
	if p.Regex != nil {
		return p.Regex.MatchString(name)
	}
	return false
}

// DedupRules ports upstream's 3 default rules verbatim. Extend by appending.
var DedupRules = []DedupeRule{
	{
		Triggers: []Pattern{exact("mcp__exa__web_search_exa"), exact("mcp__exa__web_fetch_exa")},
		Strip:    []Pattern{exact("WebSearch"), exact("WebFetch"), exact("mcp__workspace__web_fetch")},
	},
	{
		Triggers: []Pattern{exact("mcp__tavily__tavily_search"), exact("mcp__tavily__tavily_extract")},
		Strip:    []Pattern{exact("WebSearch"), exact("WebFetch"), exact("mcp__workspace__web_fetch")},
	},
	{
		Triggers: []Pattern{re(`^mcp__browsermcp__`)},
		Strip:    []Pattern{re(`^mcp__Claude_in_Chrome__`)},
	},
}

// ToolName extracts the canonical name from either OpenAI tool ({name}) or
// function-wrapped ({function:{name}}) shapes.
func ToolName(t map[string]any) string {
	if v, ok := t["name"].(string); ok && v != "" {
		return v
	}
	if fn, ok := t["function"].(map[string]any); ok {
		if v, ok := fn["name"].(string); ok {
			return v
		}
	}
	return ""
}

// DedupeTools returns (filtered, stripped-names). Empty input is passed through.
func DedupeTools(tools []map[string]any) ([]map[string]any, []string) {
	if len(tools) == 0 {
		return tools, nil
	}
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = ToolName(t)
	}
	toStrip := map[string]bool{}
	for _, rule := range DedupRules {
		triggered := false
		for _, n := range names {
			for _, p := range rule.Triggers {
				if match(n, p) {
					triggered = true
					break
				}
			}
			if triggered {
				break
			}
		}
		if !triggered {
			continue
		}
		for _, n := range names {
			for _, p := range rule.Strip {
				if match(n, p) {
					toStrip[n] = true
				}
			}
		}
	}
	if len(toStrip) == 0 {
		return tools, nil
	}
	out := make([]map[string]any, 0, len(tools))
	stripped := make([]string, 0, len(toStrip))
	for i, t := range tools {
		if toStrip[names[i]] {
			stripped = append(stripped, names[i])
			continue
		}
		out = append(out, t)
	}
	return out, stripped
}
