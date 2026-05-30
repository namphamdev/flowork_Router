// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package router

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/flowork-os/flowork_Router/internal/store"
)

func defaultCfg() store.CostRouting {
	return store.CostRouting{
		Enabled:            true,
		CheapMaxChars:      2000,
		StandardMaxChars:   10000,
		StrongOnCode:       true,
		StrongOnToolUse:    true,
		StrongMinMessages:  6,
		HonorExplicitModel: true,
	}
}

func userMsg(content string) OpenAIMessage {
	return OpenAIMessage{Role: "user", Content: content}
}

func TestClassifyCost_Cheap(t *testing.T) {
	req := OpenAIRequest{Messages: []OpenAIMessage{userMsg("what time is it?")}}
	if got := ClassifyCost(req, defaultCfg()); got != TierCheap {
		t.Fatalf("expected cheap, got %s", got)
	}
}

func TestClassifyCost_StandardFromChars(t *testing.T) {
	body := strings.Repeat("paragraph of normal text. ", 100) // ~2600 chars
	req := OpenAIRequest{Messages: []OpenAIMessage{userMsg(body)}}
	if got := ClassifyCost(req, defaultCfg()); got != TierStandard {
		t.Fatalf("expected standard, got %s", got)
	}
}

func TestClassifyCost_StrongFromChars(t *testing.T) {
	body := strings.Repeat("x", 11000) // > StandardMaxChars
	req := OpenAIRequest{Messages: []OpenAIMessage{userMsg(body)}}
	if got := ClassifyCost(req, defaultCfg()); got != TierStrong {
		t.Fatalf("expected strong, got %s", got)
	}
}

func TestClassifyCost_StrongFromCode(t *testing.T) {
	req := OpenAIRequest{Messages: []OpenAIMessage{
		userMsg("fix this:\n```go\nfunc main(){}\n```"),
	}}
	if got := ClassifyCost(req, defaultCfg()); got != TierStrong {
		t.Fatalf("code block should force strong, got %s", got)
	}
}

func TestClassifyCost_StrongFromToolUse(t *testing.T) {
	tools := json.RawMessage(`[{"type":"function","function":{"name":"x"}}]`)
	req := OpenAIRequest{
		Tools:    tools,
		Messages: []OpenAIMessage{userMsg("hi")},
	}
	if got := ClassifyCost(req, defaultCfg()); got != TierStrong {
		t.Fatalf("tools should force strong, got %s", got)
	}
}

func TestClassifyCost_StrongFromMultiTurn(t *testing.T) {
	var msgs []OpenAIMessage
	for i := 0; i < 6; i++ {
		msgs = append(msgs, userMsg("turn"))
	}
	req := OpenAIRequest{Messages: msgs}
	if got := ClassifyCost(req, defaultCfg()); got != TierStrong {
		t.Fatalf("≥6 messages should force strong, got %s", got)
	}
}

func TestClassifyCost_CodeRuleRespectsToggle(t *testing.T) {
	cfg := defaultCfg()
	cfg.StrongOnCode = false
	req := OpenAIRequest{Messages: []OpenAIMessage{userMsg("```js\n1\n```")}}
	if got := ClassifyCost(req, cfg); got != TierCheap {
		t.Fatalf("StrongOnCode=false should not bump; got %s", got)
	}
}

func TestFilterByTier_KeepsTagged(t *testing.T) {
	matches := []store.ProviderConnection{
		{ID: "a", Data: map[string]any{"tags": []any{"tier:cheap"}}},
		{ID: "b", Data: map[string]any{"tags": []any{"tier:standard"}}},
		{ID: "c", Data: map[string]any{"tags": []any{"tier:cheap", "local"}}},
	}
	out := filterByTier(matches, TierCheap)
	if len(out) != 2 || out[0].ID != "a" || out[1].ID != "c" {
		t.Fatalf("expected [a,c], got %+v", out)
	}
}

func TestFilterByTier_EmptyOnNoMatch(t *testing.T) {
	matches := []store.ProviderConnection{
		{ID: "a", Data: map[string]any{"tags": []any{"tier:strong"}}},
	}
	if out := filterByTier(matches, TierCheap); len(out) != 0 {
		t.Fatalf("expected empty result, got %+v", out)
	}
}

func TestHasActiveProviderForModel(t *testing.T) {
	matches := []store.ProviderConnection{
		{Data: map[string]any{store.CfgModels: []any{"claude-sonnet-4-5"}}},
	}
	if !hasActiveProviderForModel(matches, "claude-sonnet-4-5") {
		t.Fatal("should find exact match")
	}
	if hasActiveProviderForModel(matches, "gpt-4o-mini") {
		t.Fatal("should not match unknown model")
	}
	if hasActiveProviderForModel(matches, "") {
		t.Fatal("empty model should never match")
	}
}
