package streamutil

import (
	"strings"
	"testing"
)

func TestDedupeTools_ExaStripsBuiltinWeb(t *testing.T) {
	tools := []map[string]any{
		{"name": "mcp__exa__web_search_exa"},
		{"name": "mcp__exa__web_fetch_exa"},
		{"name": "WebSearch"},
		{"name": "WebFetch"},
		{"name": "ReadFile"},
	}
	out, stripped := DedupeTools(tools)
	if len(out) != 3 {
		t.Fatalf("expected 3 tools left (exa-search, exa-fetch, ReadFile), got %d: %v", len(out), out)
	}
	if !strings.Contains(strings.Join(stripped, ","), "WebSearch") ||
		!strings.Contains(strings.Join(stripped, ","), "WebFetch") {
		t.Fatalf("WebSearch/WebFetch should have been stripped, got %v", stripped)
	}
}

func TestDedupeTools_NoTriggerLeavesAlone(t *testing.T) {
	tools := []map[string]any{
		{"name": "WebSearch"},
		{"name": "ReadFile"},
	}
	out, stripped := DedupeTools(tools)
	if len(out) != 2 || len(stripped) != 0 {
		t.Fatalf("no Exa/Tavily trigger present → must keep both tools; got out=%v stripped=%v", out, stripped)
	}
}

func TestDedupeTools_RegexTrigger(t *testing.T) {
	tools := []map[string]any{
		{"name": "mcp__browsermcp__navigate"},
		{"name": "mcp__Claude_in_Chrome__click"},
		{"name": "ReadFile"},
	}
	out, _ := DedupeTools(tools)
	for _, t2 := range out {
		if n, _ := t2["name"].(string); strings.HasPrefix(n, "mcp__Claude_in_Chrome__") {
			t.Fatalf("Claude-in-Chrome tools must be stripped when browsermcp present, got %v", out)
		}
	}
}

func TestSessionManager_StableWithinProcess(t *testing.T) {
	id1 := DeriveSessionID("conn-1")
	id2 := DeriveSessionID("conn-1")
	if id1 != id2 || id1 == "" {
		t.Fatalf("same connection must yield stable id; got %q vs %q", id1, id2)
	}
	if DeriveSessionID("conn-2") == id1 {
		t.Fatal("different connections must yield different ids")
	}
	ResetSessionID("conn-1")
	if DeriveSessionID("conn-1") == id1 {
		t.Fatal("ResetSessionID should produce a fresh id")
	}
}

func TestClaudeHeaderCache_OnlyCaptureFromCliClient(t *testing.T) {
	CaptureFromRequest(map[string]string{
		"user-agent": "curl/8.0",
		"anthropic-beta": "tools-2024-04-04",
	})
	if HasCachedClaudeHeaders() {
		t.Fatal("non-CLI client must not populate cache")
	}
	CaptureFromRequest(map[string]string{
		"user-agent":     "claude-cli/1.0.0",
		"anthropic-beta": "tools-2024-04-04",
		"x-app":          "cli",
	})
	if !HasCachedClaudeHeaders() {
		t.Fatal("CLI client headers must populate the cache")
	}
	got := GetCachedClaudeHeaders()
	if got["anthropic-beta"] != "tools-2024-04-04" {
		t.Fatalf("anthropic-beta not captured: %v", got)
	}
}

func TestReasoningInjector_DeepseekAllScope(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{"role": "assistant", "content": "hello"},
		},
	}
	out := InjectReasoningContent("deepseek", "deepseek-chat", body)
	msgs := out["messages"].([]any)
	asst := msgs[1].(map[string]any)
	if rc, _ := asst["reasoning_content"].(string); rc == "" {
		t.Fatalf("assistant message must get reasoning_content placeholder, got: %v", asst)
	}
}

func TestReasoningInjector_KimiToolCallsOnly(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{"role": "assistant", "content": "plain reply"},
			map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "t1"}}},
		},
	}
	out := InjectReasoningContent("kimi", "kimi-thinking", body)
	msgs := out["messages"].([]any)
	plain := msgs[0].(map[string]any)
	withTools := msgs[1].(map[string]any)
	if rc, _ := plain["reasoning_content"].(string); rc != "" {
		t.Fatalf("plain assistant must NOT get injection under toolCalls scope")
	}
	if rc, _ := withTools["reasoning_content"].(string); rc == "" {
		t.Fatalf("assistant with tool_calls must get injection")
	}
}
