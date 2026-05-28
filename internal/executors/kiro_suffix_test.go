package executors

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestIsKiroAgenticModel(t *testing.T) {
	cases := map[string]bool{
		"claude-sonnet-4":                  false,
		"claude-sonnet-4-agentic":          true,
		"claude-sonnet-4-thinking":         false,
		"claude-sonnet-4-thinking-agentic": true,
		"claude-sonnet-4-agentic-thinking": true,
	}
	for in, want := range cases {
		if got := IsKiroAgenticModel(in); got != want {
			t.Errorf("IsKiroAgenticModel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsKiroThinkingModel(t *testing.T) {
	cases := map[string]bool{
		"claude-sonnet-4":                  false,
		"claude-sonnet-4-thinking":         true,
		"claude-sonnet-4-agentic":          false,
		"claude-sonnet-4-thinking-agentic": true,
		"claude-sonnet-4-agentic-thinking": true,
	}
	for in, want := range cases {
		if got := IsKiroThinkingModel(in); got != want {
			t.Errorf("IsKiroThinkingModel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestResolveKiroModel_StripsAllSuffixes(t *testing.T) {
	cases := map[string]string{
		"claude-sonnet-4":                  "claude-sonnet-4",
		"claude-sonnet-4-agentic":          "claude-sonnet-4",
		"claude-sonnet-4-thinking":         "claude-sonnet-4",
		"claude-sonnet-4-thinking-agentic": "claude-sonnet-4",
		"claude-sonnet-4-agentic-thinking": "claude-sonnet-4",
		"plain":                            "plain",
	}
	for in, want := range cases {
		if got := ResolveKiroModel(in); got != want {
			t.Errorf("ResolveKiroModel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKiroBody_StripsSuffixBeforeSending(t *testing.T) {
	body := kiroBody(Request{
		Model: "claude-sonnet-4-agentic",
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
	})
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	cs, _ := parsed["conversationState"].(map[string]any)
	if mid, _ := cs["modelId"].(string); mid != "claude-sonnet-4" {
		t.Fatalf("upstream modelId must be base, got %q", mid)
	}
	// The body should also carry the agentic system prompt in history[0].
	if !bytes.Contains(body, []byte("CHUNKED WRITE PROTOCOL")) {
		t.Fatal("agentic system prompt not injected")
	}
}

func TestKiroBody_NonAgenticDoesNotInjectPrompt(t *testing.T) {
	body := kiroBody(Request{
		Model:    "claude-sonnet-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if bytes.Contains(body, []byte("CHUNKED WRITE PROTOCOL")) {
		t.Fatal("non-agentic must NOT carry chunked-write prompt")
	}
}

func TestKiroBody_ThinkingOnlyDoesNotInjectAgenticPrompt(t *testing.T) {
	body := kiroBody(Request{
		Model:    "claude-sonnet-4-thinking",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if bytes.Contains(body, []byte("CHUNKED WRITE PROTOCOL")) {
		t.Fatal("thinking-only must NOT carry agentic prompt")
	}
}

func TestKiroAgenticSystemPrompt_NonEmpty(t *testing.T) {
	if !strings.Contains(KiroAgenticSystemPrompt, "MAXIMUM 350 LINES") {
		t.Fatal("agentic prompt missing core rule")
	}
}
