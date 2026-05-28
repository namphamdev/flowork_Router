package router

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// TestBrainOllamaE2E is the REAL end-to-end: an agent request to the brain model
// flows through the dispatcher, gets enriched, and is answered by a REAL Ollama
// model (flowork-brain). Proves the last mile (live inference), not a mock.
// Gated: only runs with OLLAMA_E2E=1, a temp FLOW_ROUTER_DATA, and a brain DB.
// Enrichment is kept minimal (topK=1, no skills) because the brain gguf runs
// CPU-split on a low-VRAM box and prompt-eval is slow.
//	OLLAMA_E2E=1 FLOW_ROUTER_DATA=$(mktemp -d) \
//	FLOW_ROUTER_BRAIN_DB=/path/brain.sqlite \
//	  go test ./internal/router/ -run BrainOllamaE2E -v -timeout 600s
func TestBrainOllamaE2E(t *testing.T) {
	if os.Getenv("OLLAMA_E2E") != "1" {
		t.Skip("set OLLAMA_E2E=1 to run the live Ollama e2e")
	}
	brainDB := os.Getenv("FLOW_ROUTER_BRAIN_DB")
	if brainDB == "" {
		t.Skip("FLOW_ROUTER_BRAIN_DB not set")
	}
	data := os.Getenv("FLOW_ROUTER_DATA")
	if data == "" || strings.HasPrefix(data, os.Getenv("HOME")+"/.flow_router") {
		t.Skip("set FLOW_ROUTER_DATA to a temp dir to run safely")
	}

	d, err := store.Open()
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	if err := store.UpsertProvider(d, &store.ProviderConnection{
		ID: "ollama-brain", Provider: "openai", AuthType: "none", Name: "ollama", IsActive: true,
		Data: map[string]any{
			store.CfgBaseURL: "http://127.0.0.1:11434/v1",
			store.CfgFormat:  "openai",
			store.CfgModels:  []any{"flowork-brain"},
		},
	}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	s, _ := store.LoadSettings(d)
	s.Brain = store.BrainConfig{
		Enabled: true, Model: "flowork-brain", DBPath: brainDB, Mode: "augment",
		TopK: 1, MaxSnippetChars: 200, Skills: false, Record: true,
	}
	if err := store.SaveSettings(d, s); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	resp, status, err := DispatchChatCompletion(context.Background(), OpenAIRequest{
		Model:     "flowork-brain",
		MaxTokens: 24,
		Messages:  []OpenAIMessage{{Role: "user", Content: "what is sql injection in one sentence"}},
	})
	if err != nil || status != 200 {
		t.Fatalf("dispatch: status=%d err=%v", status, err)
	}
	if resp == nil || len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		t.Fatalf("empty model response: %+v", resp)
	}
	total, _ := store.CountBrainContributions(d)
	t.Logf("LIVE OLLAMA e2e OK: model=%s answer=%q contributions=%d",
		resp.Model, truncate(resp.Choices[0].Message.Content, 200), total)
}
