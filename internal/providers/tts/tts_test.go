// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file. Standard test patterns, no production race/leak risk.

package tts

import "testing"

func TestTTSProviders_AllRegistered(t *testing.T) {
	want := []string{
		"openai", "elevenlabs", "gemini", "googleTts", "edgeTts",
		"localDevice", "minimax", "openrouter", "deepgram", "inworld",
	}
	for _, n := range want {
		if Get(n) == nil {
			t.Fatalf("TTS provider %q not registered", n)
		}
	}
	if len(List()) < len(want) {
		t.Fatalf("List returned %d, expected ≥%d", len(List()), len(want))
	}
}
