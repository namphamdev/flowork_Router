package providercompat

import "testing"

func TestIsOpenAICompatible(t *testing.T) {
	cases := map[string]bool{
		"":                                false,
		"openai":                          false,
		"openai-compatible-groq":          true,
		"openai-compatible-fireworks":     true,
		"openai-compatible-x-responses":   true,
		"anthropic-compatible-aws":        false,
	}
	for in, want := range cases {
		if got := IsOpenAICompatible(in); got != want {
			t.Errorf("IsOpenAICompatible(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsAnthropicCompatible(t *testing.T) {
	cases := map[string]bool{
		"anthropic":                      false,
		"anthropic-compatible-aws":       true,
		"anthropic-compatible-vertex":    true,
		"openai-compatible-x":            false,
		"":                               false,
	}
	for in, want := range cases {
		if got := IsAnthropicCompatible(in); got != want {
			t.Errorf("IsAnthropicCompatible(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestOpenAIAPIType(t *testing.T) {
	cases := map[string]string{
		"openai-compatible-chatlike":      "chat",
		"openai-compatible-x-responses":   "responses",
		"openai-compatible-responses-x":   "responses", // suffix anywhere triggers
		"anthropic-compatible-aws":        "",
		"":                                "",
	}
	for in, want := range cases {
		if got := OpenAIAPIType(in); got != want {
			t.Errorf("OpenAIAPIType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildOpenAICompatURL_ChatVsResponses(t *testing.T) {
	if got := BuildOpenAICompatURL("https://api.foo.com/v1", "chat"); got != "https://api.foo.com/v1/chat/completions" {
		t.Errorf("chat: %s", got)
	}
	if got := BuildOpenAICompatURL("https://api.foo.com/v1", "responses"); got != "https://api.foo.com/v1/responses" {
		t.Errorf("responses: %s", got)
	}
}

func TestBuildOpenAICompatURL_TrailingSlashNormalised(t *testing.T) {
	if got := BuildOpenAICompatURL("https://api.foo.com/v1/", "chat"); got != "https://api.foo.com/v1/chat/completions" {
		t.Errorf("trailing slash leaked: %s", got)
	}
}

func TestBuildOpenAICompatURL_EmptyBaseUsesDefault(t *testing.T) {
	if got := BuildOpenAICompatURL("", "chat"); got != "https://api.openai.com/v1/chat/completions" {
		t.Errorf("empty base default wrong: %s", got)
	}
}

func TestBuildAnthropicCompatURL(t *testing.T) {
	if got := BuildAnthropicCompatURL("https://api.foo.com/v1"); got != "https://api.foo.com/v1/messages" {
		t.Errorf("got %s", got)
	}
	if got := BuildAnthropicCompatURL(""); got != "https://api.anthropic.com/v1/messages" {
		t.Errorf("empty default wrong: %s", got)
	}
}

func TestResolveFormat(t *testing.T) {
	cases := map[string]string{
		"openai-compatible-x":           "openai",
		"openai-compatible-x-responses": "openai-responses",
		"anthropic-compatible-aws":      "anthropic",
		"openai":                        "", // no prefix → caller falls back
		"custom":                        "",
	}
	for in, want := range cases {
		if got := ResolveFormat(in); got != want {
			t.Errorf("ResolveFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveBaseURL_ExplicitWins(t *testing.T) {
	if got := ResolveBaseURL("openai-compatible-x", "https://groq.com/v1/"); got != "https://groq.com/v1" {
		t.Errorf("explicit base lost: %s", got)
	}
}

func TestResolveBaseURL_FallsBackToVendorDefault(t *testing.T) {
	if got := ResolveBaseURL("openai-compatible-x", ""); got != "https://api.openai.com/v1" {
		t.Errorf("openai default: %s", got)
	}
	if got := ResolveBaseURL("anthropic-compatible-x", ""); got != "https://api.anthropic.com/v1" {
		t.Errorf("anthropic default: %s", got)
	}
}

func TestResolveBaseURL_EmptyForUnknown(t *testing.T) {
	if got := ResolveBaseURL("custom-vendor", ""); got != "" {
		t.Errorf("unknown should return empty: %s", got)
	}
}
