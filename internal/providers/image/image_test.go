package image

import "testing"

func TestImageProviders_AllRegistered(t *testing.T) {
	want := []string{
		"openai", "blackForestLabs", "cloudflareAi", "codex", "comfyui",
		"falAi", "gemini", "huggingface", "nanobanana", "runwayml",
		"sdwebui", "stabilityAi",
	}
	for _, n := range want {
		if Get(n) == nil {
			t.Fatalf("image provider %q not registered", n)
		}
	}
	names := List()
	if len(names) < len(want) {
		t.Fatalf("List returned %d names, expected ≥%d", len(names), len(want))
	}
}

func TestSplitSize(t *testing.T) {
	cases := map[string]struct{ w, h int }{
		"1024x1024": {1024, 1024},
		"512x768":   {512, 768},
		"":          {1024, 1024},
		"broken":    {1024, 1024},
	}
	for in, want := range cases {
		w, h := splitSize(in, 1024)
		if w != want.w || h != want.h {
			t.Fatalf("%q → (%d,%d), want (%d,%d)", in, w, h, want.w, want.h)
		}
	}
}
