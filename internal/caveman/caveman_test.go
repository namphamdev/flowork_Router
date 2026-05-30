// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package caveman

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := map[string]Level{
		"":         LevelOff,
		"  ":       LevelOff,
		"off":      LevelOff,
		"lite":     LevelLite,
		"LITE":     LevelLite,
		"  Full ":  LevelFull,
		"ultra":    LevelUltra,
		"bogus":    LevelOff,
		"thinking": LevelOff,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPromptContent(t *testing.T) {
	// Each level returns a non-empty prompt and carries the shared boundaries.
	for _, l := range []Level{LevelLite, LevelFull, LevelUltra} {
		p := Prompt(l)
		if p == "" {
			t.Fatalf("Prompt(%s) empty", l)
		}
		if !strings.Contains(p, "Code blocks") {
			t.Errorf("Prompt(%s) missing boundary protection clause", l)
		}
		if !strings.Contains(p, "Active every response until user asks for normal mode") {
			t.Errorf("Prompt(%s) missing persistence clause", l)
		}
	}
}

func TestPrompt_OffReturnsEmpty(t *testing.T) {
	if p := Prompt(LevelOff); p != "" {
		t.Fatalf("Prompt(off) = %q, want empty", p)
	}
}

func TestInjectIntoSystem_AppendWhenExisting(t *testing.T) {
	got := InjectIntoSystem("You are X.", "Respond tersely.")
	if got != "You are X.\n\nRespond tersely." {
		t.Fatalf("append mismatch: %q", got)
	}
}

func TestInjectIntoSystem_PreservesEmpty(t *testing.T) {
	if got := InjectIntoSystem("hello", ""); got != "hello" {
		t.Fatalf("empty prompt should be no-op: %q", got)
	}
	if got := InjectIntoSystem("", "modifier"); got != "modifier" {
		t.Fatalf("empty existing should return modifier: %q", got)
	}
}

func TestPromptsAreDistinctAcrossLevels(t *testing.T) {
	lite := Prompt(LevelLite)
	full := Prompt(LevelFull)
	ultra := Prompt(LevelUltra)
	if lite == full || full == ultra || lite == ultra {
		t.Fatal("levels should not share prompt text")
	}
}
