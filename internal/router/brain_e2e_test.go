package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// TestBrainE2E proves the full path: a request to the brain model flows through
// the real dispatcher, gets enriched, and the INJECTED knowledge actually
// reaches the upstream provider. Uses a mock OpenAI upstream (no Ollama needed).
// Run isolated so the process-wide store handle points at a throwaway DB:
//	FLOW_ROUTER_DATA=$(mktemp -d) FLOW_ROUTER_BRAIN_DB=/path/brain.sqlite \
//	  go test ./internal/router/ -run BrainE2E -v
// Skips unless both a temp FLOW_ROUTER_DATA and a real brain DB are provided,
// so it never touches the real flow_router DB.
func TestBrainE2E(t *testing.T) {
	brainDB := os.Getenv("FLOW_ROUTER_BRAIN_DB")
	if brainDB == "" {
		t.Skip("FLOW_ROUTER_BRAIN_DB not set — skipping brain e2e")
	}
	data := os.Getenv("FLOW_ROUTER_DATA")
	if data == "" || strings.HasPrefix(data, os.Getenv("HOME")+"/.flow_router") {
		t.Skip("set FLOW_ROUTER_DATA to a temp dir to run brain e2e safely")
	}

	// Mock OpenAI-compatible upstream that captures the forwarded request.
	var captured OpenAIRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_ = json.NewEncoder(w).Encode(OpenAIResponse{
			ID: "cmpl-test", Object: "chat.completion", Model: "flowork-brain",
			Choices: []OpenAIChoice{{Index: 0, FinishReason: "stop",
				Message: OpenAIMessage{Role: "assistant", Content: "ok"}}},
		})
	}))
	defer srv.Close()

	d, err := store.Open()
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	if err := store.UpsertProvider(d, &store.ProviderConnection{
		ID: "test-brain-backend", Provider: "openai", AuthType: "none", Name: "mock", IsActive: true,
		Data: map[string]any{
			store.CfgBaseURL: srv.URL,
			store.CfgFormat:  "openai",
			store.CfgAPIKey:  "test",
			store.CfgModels:  []any{"flowork-brain"},
		},
	}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	s, _ := store.LoadSettings(d)
	s.Brain = store.BrainConfig{
		Enabled: true, Model: "flowork-brain", DBPath: brainDB, Mode: "augment",
		TopK: 3, MaxSnippetChars: 300, Skills: true, SkillTopK: 1, Record: true,
	}
	if err := store.SaveSettings(d, s); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	beforeTotal, _ := store.CountBrainContributions(d)

	resp, status, err := DispatchChatCompletion(context.Background(), OpenAIRequest{
		Model:    "flowork-brain",
		Messages: []OpenAIMessage{{Role: "user", Content: "how to bypass a waf for sql injection"}},
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("dispatch: status=%d err=%v", status, err)
	}
	if resp == nil || len(resp.Choices) == 0 {
		t.Fatal("empty response")
	}

	// The upstream must have received the injected brain knowledge.
	var sysBlock string
	for _, m := range captured.Messages {
		if m.Role == "system" {
			sysBlock += m.Content
		}
	}
	if !strings.Contains(sysBlock, "Relevant knowledge") {
		t.Fatalf("upstream did not receive injected knowledge; system block = %q", truncate(sysBlock, 300))
	}

	// Compounding: the interaction must have been queued as a contribution.
	afterTotal, pending := store.CountBrainContributions(d)
	if afterTotal != beforeTotal+1 {
		t.Fatalf("expected 1 new contribution, total %d → %d", beforeTotal, afterTotal)
	}
	contribs, _ := store.ListBrainContributions(d, true, 5)
	if len(contribs) == 0 || contribs[0].Answer != "ok" || contribs[0].Query == "" {
		t.Fatalf("contribution not recorded correctly: %+v", contribs)
	}
	t.Logf("e2e OK: upstream saw %d messages, system block %d chars, contributions total=%d pending=%d answer=%q",
		len(captured.Messages), len(sysBlock), afterTotal, pending, contribs[0].Answer)
}
