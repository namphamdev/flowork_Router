// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package main

import (
	"testing"
)

func TestInferMiniMaxLanguage(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"English_FluentMan", "English"},
		{"Chinese_StoicWoman", "Chinese"},
		{"Spanish_Narrator", "Spanish"},
		{"NoUnderscore", "Custom"},
		{"", "Custom"},
		{"_LeadingUnderscore", "Custom"},
		{"   trim   ", "Custom"},
	}
	for _, c := range cases {
		got := inferMiniMaxLanguage(c.in)
		if got != c.want {
			t.Errorf("inferMiniMaxLanguage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVoiceEnvelope_AddVoice_Dedupes(t *testing.T) {
	env := &voiceEnvelope{}
	env.addVoice("en", voiceRec{ID: "v1", Name: "Voice One"})
	env.addVoice("en", voiceRec{ID: "v1", Name: "Voice One (duplicate)"})
	env.addVoice("en", voiceRec{ID: "v2", Name: "Voice Two"})
	if len(env.ByLang["en"].Voices) != 2 {
		t.Fatalf("expected 2 voices after dedupe, got %d", len(env.ByLang["en"].Voices))
	}
	if env.ByLang["en"].Voices[0].Name != "Voice One" {
		t.Errorf("first insert should win for dedupe, got %q", env.ByLang["en"].Voices[0].Name)
	}
}

func TestVoiceEnvelope_Finalize_CustomLast(t *testing.T) {
	env := &voiceEnvelope{}
	env.addVoice("Custom", voiceRec{ID: "c1", Name: "Cloned"})
	env.addVoice("en", voiceRec{ID: "e1", Name: "English"})
	env.addVoice("de", voiceRec{ID: "d1", Name: "Deutsch"})
	env.finalize()
	if len(env.Languages) != 3 {
		t.Fatalf("expected 3 languages, got %d", len(env.Languages))
	}
	last := env.Languages[len(env.Languages)-1]
	if last.Code != "Custom" {
		t.Errorf("Custom should sort last, got %s last", last.Code)
	}
	// First two should be sorted alphabetically (de, en)
	if env.Languages[0].Code != "de" || env.Languages[1].Code != "en" {
		t.Errorf("non-custom should sort alphabetically: got %s, %s", env.Languages[0].Code, env.Languages[1].Code)
	}
}

func TestVoiceEnvelope_Finalize_SortsVoicesByName(t *testing.T) {
	env := &voiceEnvelope{}
	env.addVoice("en", voiceRec{ID: "v3", Name: "Charlie"})
	env.addVoice("en", voiceRec{ID: "v1", Name: "Alice"})
	env.addVoice("en", voiceRec{ID: "v2", Name: "Bob"})
	env.finalize()
	got := env.Languages[0].Voices
	if got[0].Name != "Alice" || got[1].Name != "Bob" || got[2].Name != "Charlie" {
		t.Errorf("voices not sorted by name: %v", got)
	}
}

func TestNormalizeMiniMaxVoices_AllGroups(t *testing.T) {
	data := map[string]any{
		"system_voice": []any{
			map[string]any{"voice_id": "English_Anchor", "voice_name": "Anchor"},
			map[string]any{"voice_id": "Chinese_Reader", "voice_name": "Reader"},
		},
		"voice_cloning": []any{
			map[string]any{"voice_id": "clone_abc", "voice_name": "MyClone"},
		},
		"voice_generation": []any{
			map[string]any{"voice_id": "gen_xyz", "voice_name": "Generated"},
		},
		"music_generation": []any{
			map[string]any{"voice_id": "music_001"},
		},
	}
	env := normalizeMiniMaxVoices(data)
	env.finalize()

	// English bucket should hold "Anchor"
	if b, ok := env.ByLang["English"]; !ok || len(b.Voices) != 1 || b.Voices[0].ID != "English_Anchor" {
		t.Errorf("English bucket wrong: %+v", env.ByLang["English"])
	}
	// Chinese bucket should hold "Reader"
	if b, ok := env.ByLang["Chinese"]; !ok || len(b.Voices) != 1 {
		t.Errorf("Chinese bucket wrong: %+v", env.ByLang["Chinese"])
	}
	// Custom should aggregate all 3 non-system groups
	customs, ok := env.ByLang["Custom"]
	if !ok || len(customs.Voices) != 3 {
		t.Fatalf("Custom bucket should hold 3 cloned/generated/music, got %+v", customs)
	}
	// Cloned voice should have " · Cloned" suffix
	foundCloned := false
	for _, v := range customs.Voices {
		if v.ID == "clone_abc" && v.Name == "MyClone · Cloned" {
			foundCloned = true
		}
	}
	if !foundCloned {
		t.Errorf("cloned voice missing group-label suffix: %+v", customs.Voices)
	}
}

func TestNormalizeMiniMaxVoices_EmptyData(t *testing.T) {
	env := normalizeMiniMaxVoices(map[string]any{})
	env.finalize()
	if len(env.Languages) != 0 {
		t.Errorf("empty data should produce empty envelope, got %d langs", len(env.Languages))
	}
}

func TestNormalizeMiniMaxVoices_SkipsMissingID(t *testing.T) {
	data := map[string]any{
		"system_voice": []any{
			map[string]any{"voice_name": "no-id"},
			map[string]any{"voice_id": "", "voice_name": "empty-id"},
			map[string]any{"voice_id": "English_OK", "voice_name": "Valid"},
		},
	}
	env := normalizeMiniMaxVoices(data)
	env.finalize()
	total := 0
	for _, b := range env.Languages {
		total += len(b.Voices)
	}
	if total != 1 {
		t.Errorf("expected only 1 valid voice, got %d total", total)
	}
}

func TestFirstStrTTS_PicksFirstNonEmpty(t *testing.T) {
	if got := firstStrTTS("", "", "third", "fourth"); got != "third" {
		t.Errorf("expected 'third', got %q", got)
	}
	if got := firstStrTTS("first", "second"); got != "first" {
		t.Errorf("expected 'first', got %q", got)
	}
	if got := firstStrTTS("", "", ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := firstStrTTS(); got != "" {
		t.Errorf("no args should return empty, got %q", got)
	}
}
