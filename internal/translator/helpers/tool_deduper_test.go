package helpers

import (
	"sort"
	"testing"
)

// Build a tool list with the given names in OpenAI shape so we exercise the
// `function.name` extraction path.
func makeOpenAITools(names ...string) []any {
	out := make([]any, len(names))
	for i, n := range names {
		out[i] = map[string]any{
			"type": "function",
			"function": map[string]any{"name": n},
		}
	}
	return out
}

// Anthropic shape uses a top-level "name" field instead of nested function.
func makeAnthropicTools(names ...string) []any {
	out := make([]any, len(names))
	for i, n := range names {
		out[i] = map[string]any{"name": n}
	}
	return out
}

func TestDedupeTools_NoMCPLeavesUntouched(t *testing.T) {
	in := makeOpenAITools("WebSearch", "WebFetch", "custom_tool")
	out, stripped := DedupeTools(in)
	if len(stripped) != 0 {
		t.Fatalf("nothing should strip without MCP trigger, got %v", stripped)
	}
	if len(out) != 3 {
		t.Fatalf("list should be unchanged, got %d", len(out))
	}
}

func TestDedupeTools_ExaTriggersBuiltInStrip(t *testing.T) {
	in := makeOpenAITools(
		"mcp__exa__web_search_exa",
		"WebSearch",
		"WebFetch",
		"my_custom_tool",
	)
	out, stripped := DedupeTools(in)
	got := map[string]bool{}
	for _, s := range stripped {
		got[s] = true
	}
	for _, want := range []string{"WebSearch", "WebFetch"} {
		if !got[want] {
			t.Errorf("expected %q in stripped list, got %v", want, stripped)
		}
	}
	for _, tool := range out {
		name := extractToolName(tool)
		if name == "WebSearch" || name == "WebFetch" {
			t.Errorf("%q should have been removed", name)
		}
	}
}

func TestDedupeTools_TavilyTriggersBuiltInStrip(t *testing.T) {
	in := makeOpenAITools(
		"mcp__tavily__tavily_search",
		"WebSearch",
		"mcp__workspace__web_fetch",
	)
	_, stripped := DedupeTools(in)
	sort.Strings(stripped)
	want := []string{"WebSearch", "mcp__workspace__web_fetch"}
	sort.Strings(want)
	if len(stripped) != len(want) {
		t.Fatalf("expected 2 stripped tools, got %v", stripped)
	}
	for i := range want {
		if stripped[i] != want[i] {
			t.Errorf("stripped[%d] = %q, want %q", i, stripped[i], want[i])
		}
	}
}

func TestDedupeTools_BrowserMCPStripsClaudeInChrome(t *testing.T) {
	in := makeOpenAITools(
		"mcp__browsermcp__browser_click",
		"mcp__browsermcp__browser_snapshot",
		"mcp__Claude_in_Chrome__open_tab",
		"mcp__Claude_in_Chrome__capture",
		"WebSearch", // should NOT be stripped (no exa/tavily trigger)
	)
	out, stripped := DedupeTools(in)
	for _, s := range stripped {
		if s == "WebSearch" {
			t.Fatal("WebSearch should NOT be stripped without exa/tavily trigger")
		}
	}
	names := map[string]bool{}
	for _, t := range out {
		names[extractToolName(t)] = true
	}
	if names["mcp__Claude_in_Chrome__open_tab"] || names["mcp__Claude_in_Chrome__capture"] {
		t.Fatal("Claude_in_Chrome tools should be stripped when browsermcp is present")
	}
}

func TestDedupeTools_HandlesAnthropicShape(t *testing.T) {
	in := makeAnthropicTools(
		"mcp__exa__web_search_exa",
		"WebSearch",
	)
	out, stripped := DedupeTools(in)
	if len(stripped) != 1 || stripped[0] != "WebSearch" {
		t.Fatalf("anthropic-shape strip wrong: %v", stripped)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 tool remaining, got %d", len(out))
	}
}

func TestDedupeTools_EmptyInput(t *testing.T) {
	if out, stripped := DedupeTools(nil); out != nil || stripped != nil {
		t.Fatalf("nil input should pass through: out=%v stripped=%v", out, stripped)
	}
}

func TestDedupeTools_PreservesOrder(t *testing.T) {
	in := makeOpenAITools(
		"first",
		"mcp__exa__web_search_exa",
		"WebSearch", // stripped
		"last",
	)
	out, _ := DedupeTools(in)
	if extractToolName(out[0]) != "first" {
		t.Errorf("first lost: %v", out[0])
	}
	if extractToolName(out[len(out)-1]) != "last" {
		t.Errorf("last lost: %v", out[len(out)-1])
	}
}

func TestDedupeTools_SkipsToolsWithoutName(t *testing.T) {
	// A malformed entry shouldn't panic; just gets ignored.
	in := []any{
		map[string]any{}, // no name
		"not-a-map",
		map[string]any{"function": map[string]any{"name": "mcp__exa__web_search_exa"}},
		map[string]any{"function": map[string]any{"name": "WebSearch"}},
	}
	out, stripped := DedupeTools(in)
	if len(stripped) != 1 || stripped[0] != "WebSearch" {
		t.Fatalf("malformed entries should be ignored, expected WebSearch strip only, got %v", stripped)
	}
	if len(out) != 3 { // empty map + string + exa tool kept; WebSearch stripped
		t.Fatalf("expected 3 entries, got %d", len(out))
	}
}
