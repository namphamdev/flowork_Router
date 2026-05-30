// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package services

import (
	"testing"
	"time"
)

func TestLeadFor_KnownProviders(t *testing.T) {
	cases := map[string]time.Duration{
		"codex":       5 * 24 * time.Hour,
		"openai":      5 * 24 * time.Hour,
		"claude":      4 * time.Hour,
		"anthropic":   4 * time.Hour,
		"iflow":       24 * time.Hour,
		"qwen":        20 * time.Minute,
		"kimi":        5 * time.Minute,
		"kimi-coding": 5 * time.Minute,
		"antigravity": 5 * time.Minute,
		"gemini-cli":  5 * time.Minute,
		"github":      4 * time.Hour,
		"copilot":     4 * time.Hour,
		"kiro":        4 * time.Hour,
	}
	for provider, want := range cases {
		if got := leadFor(provider); got != want {
			t.Errorf("leadFor(%q) = %v, want %v", provider, got, want)
		}
	}
}

func TestLeadFor_UnknownFallsBackToDefault(t *testing.T) {
	old := RefreshLead
	defer func() { RefreshLead = old }()
	RefreshLead = 7 * time.Minute

	if got := leadFor("totally-new-vendor"); got != 7*time.Minute {
		t.Fatalf("unknown provider should fall back to RefreshLead, got %v", got)
	}
}

func TestLeadFor_EmptyProviderIDFallsBack(t *testing.T) {
	if leadFor("") != RefreshLead {
		t.Fatal("empty provider id should fall back to default")
	}
}

func TestLeadFor_RuntimeMutationVisible(t *testing.T) {
	// A caller can override per-provider lead at runtime by writing into
	// RefreshLeadByProvider. Verify the lookup picks up the new value.
	orig, hadOrig := RefreshLeadByProvider["unit-test-vendor"]
	RefreshLeadByProvider["unit-test-vendor"] = 42 * time.Second
	defer func() {
		if hadOrig {
			RefreshLeadByProvider["unit-test-vendor"] = orig
		} else {
			delete(RefreshLeadByProvider, "unit-test-vendor")
		}
	}()
	if got := leadFor("unit-test-vendor"); got != 42*time.Second {
		t.Fatalf("runtime override not honoured: %v", got)
	}
}
