package router

import (
	"context"
	"strings"
	"testing"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// alwaysOnSettings is a brain config with AlwaysOn=true and constitution
// injection on, ready for tests that don't care about the trigger model.
func alwaysOnSettings(dbPath string) *store.Settings {
	return &store.Settings{Brain: store.BrainConfig{
		Enabled:              true,
		Model:                "flowork-brain",
		DBPath:               dbPath,
		Mode:                 "augment",
		TopK:                 3,
		MaxSnippetChars:      300,
		Skills:               true,
		SkillTopK:            2,
		AlwaysOn:             true,
		InjectConstitution:   true,
		ConstitutionTopK:     5,
		ConstitutionMaxChars: 200,
	}}
}

func TestMaybeEnrichBrain_AlwaysOnFiresForAnyModel(t *testing.T) {
	db := realBrainDB(t)
	for _, model := range []string{"claude-sonnet-4-5", "gpt-4o-mini", "gemini-1.5-pro"} {
		req := userReq(model)
		info := maybeEnrichBrain(context.Background(), req, alwaysOnSettings(db))
		if info == nil {
			t.Fatalf("model %q: AlwaysOn should fire enrichment", model)
		}
	}
}

func TestMaybeEnrichBrain_AlwaysOnOff_BackToModelGate(t *testing.T) {
	db := realBrainDB(t)
	s := alwaysOnSettings(db)
	s.Brain.AlwaysOn = false
	req := userReq("claude-sonnet-4-5")
	if maybeEnrichBrain(context.Background(), req, s) != nil {
		t.Fatal("with AlwaysOn=false, non-matching model must NOT enrich")
	}
}

func TestMaybeInjectConstitution_InjectsTopRules(t *testing.T) {
	db := realBrainDB(t)
	req := userReq("any-model")
	maybeInjectConstitution(context.Background(), req, alwaysOnSettings(db))

	var sysContent string
	for _, m := range req.Messages {
		if m.Role == "system" {
			sysContent += m.Content
		}
	}
	if !strings.Contains(sysContent, "Project doctrine (sacred rules)") {
		t.Fatalf("constitution header missing — got system block: %q", truncate(sysContent, 200))
	}
}

func TestMaybeInjectConstitution_NoopWhenDisabled(t *testing.T) {
	db := realBrainDB(t)
	s := alwaysOnSettings(db)
	s.Brain.InjectConstitution = false

	req := userReq("any-model")
	before := len(req.Messages)
	maybeInjectConstitution(context.Background(), req, s)
	if len(req.Messages) != before {
		t.Fatalf("InjectConstitution=false must be a no-op, msgs went from %d → %d", before, len(req.Messages))
	}
}

func TestMaybeInjectConstitution_NoopWhenBrainDisabled(t *testing.T) {
	db := realBrainDB(t)
	s := alwaysOnSettings(db)
	s.Brain.Enabled = false

	req := userReq("any-model")
	before := len(req.Messages)
	maybeInjectConstitution(context.Background(), req, s)
	if len(req.Messages) != before {
		t.Fatalf("Brain.Enabled=false must skip constitution too, msgs went from %d → %d", before, len(req.Messages))
	}
}

func TestMaybeInjectConstitution_FailsOpenOnMissingDB(t *testing.T) {
	s := alwaysOnSettings("/nonexistent/path/to/brain.sqlite")
	req := userReq("any-model")
	before := len(req.Messages)
	maybeInjectConstitution(context.Background(), req, s)
	if len(req.Messages) != before {
		t.Fatalf("missing DB must be a no-op, msgs went from %d → %d", before, len(req.Messages))
	}
}
