// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package router

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// realBrainDB returns the live brain DB path from env, or "" to skip.
func realBrainDB(t *testing.T) string {
	p := os.Getenv("FLOW_ROUTER_BRAIN_DB")
	if p == "" {
		t.Skip("FLOW_ROUTER_BRAIN_DB not set — skipping brain enrichment live test")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("brain DB %q not found — skipping", p)
	}
	return p
}

func brainSettings(dbPath, mode string) *store.Settings {
	return &store.Settings{Brain: store.BrainConfig{
		Enabled: true, Model: "flowork-brain", DBPath: dbPath, Mode: mode,
		TopK: 3, MaxSnippetChars: 300, Skills: true, SkillTopK: 2,
	}}
}

func userReq(model string) *OpenAIRequest {
	return &OpenAIRequest{Model: model, Messages: []OpenAIMessage{
		{Role: "system", Content: "You are the caller's own assistant."},
		{Role: "user", Content: "how do I exploit sql injection"},
	}}
}

func TestEnrichAugment(t *testing.T) {
	db := realBrainDB(t)
	req := userReq("flowork-brain")
	info := maybeEnrichBrain(context.Background(), req, brainSettings(db, "augment"))
	if info == nil {
		t.Fatal("expected enrichment to apply")
	}
	if len(req.Messages) < 3 {
		t.Fatalf("expected an injected message, got %d", len(req.Messages))
	}
	// augment: caller's system stays first
	if !strings.Contains(req.Messages[0].Content, "caller's own assistant") {
		t.Errorf("augment must keep caller system first, got %q", req.Messages[0].Role)
	}
	// the injected brain system carries retrieved knowledge
	joined := ""
	for _, m := range req.Messages {
		if m.Role == "system" {
			joined += m.Content
		}
	}
	if !strings.Contains(joined, "Relevant knowledge") {
		t.Errorf("expected injected knowledge section, system block = %q", truncate(joined, 200))
	}
	t.Logf("augment: %d messages after enrichment, brain system len=%d", len(req.Messages), len(joined))
}

func TestEnrichBrainMode(t *testing.T) {
	db := realBrainDB(t)
	req := userReq("flowork-brain")
	if maybeEnrichBrain(context.Background(), req, brainSettings(db, "brain")) == nil {
		t.Fatal("expected enrichment to apply")
	}
	// brain mode: injected system is at index 0 (brain identity dominates)
	if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, "shared knowledge brain") {
		t.Errorf("brain mode must prepend brain system at index 0, got role=%q", req.Messages[0].Role)
	}
}

func TestEnrichDisabled(t *testing.T) {
	req := userReq("flowork-brain")
	before := len(req.Messages)
	s := brainSettings("", "augment")
	s.Brain.Enabled = false
	if maybeEnrichBrain(context.Background(), req, s) != nil {
		t.Error("disabled brain must not enrich")
	}
	if len(req.Messages) != before {
		t.Error("disabled brain must not mutate messages")
	}
}

func TestEnrichWrongModel(t *testing.T) {
	req := userReq("gpt-4o")
	if maybeEnrichBrain(context.Background(), req, brainSettings("/nonexistent", "augment")) != nil {
		t.Error("non-brain model must not enrich")
	}
}
