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
