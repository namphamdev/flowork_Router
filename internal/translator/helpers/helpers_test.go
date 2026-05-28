package helpers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFlattenAnthropic_StringPassthrough(t *testing.T) {
	if got := FlattenAnthropicSystem("hello"); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := FlattenAnthropicContent("hi"); got != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestFlattenAnthropic_ArrayOfTextParts(t *testing.T) {
	system := []any{
		map[string]any{"type": "text", "text": "A"},
		map[string]any{"type": "text", "text": "B"},
	}
	if got := FlattenAnthropicSystem(system); got != "A\n\nB" {
		t.Fatalf("system join: got %q", got)
	}
	content := []any{
		map[string]any{"type": "text", "text": "x"},
		map[string]any{"type": "text", "text": "y"},
	}
	if got := FlattenAnthropicContent(content); got != "xy" {
		t.Fatalf("content join: got %q", got)
	}
}

func TestMapStopReason(t *testing.T) {
	cases := map[string]string{
		"length":     "max_tokens",
		"tool_calls": "tool_use",
		"stop":       "end_turn",
		"":           "end_turn",
		"weird":      "weird",
	}
	for in, want := range cases {
		if got := MapClaudeStopReason(in); got != want {
			t.Fatalf("%q→ want %q, got %q", in, want, got)
		}
	}
}

func TestGeminiRoleAndFinish(t *testing.T) {
	if MapGeminiRole("assistant") != "model" {
		t.Fatal("assistant→model")
	}
	if MapGeminiRole("system") != "user" {
		t.Fatal("system→user")
	}
	if MapGeminiFinishReason("MAX_TOKENS") != "length" {
		t.Fatal("MAX_TOKENS→length")
	}
	if MapGeminiFinishReason("STOP") != "stop" {
		t.Fatal("STOP→stop")
	}
}

func TestCleanFunctionName(t *testing.T) {
	if got := CleanFunctionName(""); got != "_unknown" {
		t.Fatalf("empty: %q", got)
	}
	if got := CleanFunctionName("9foo"); got != "_9foo" {
		t.Fatalf("digit-leader must be prefixed: %q", got)
	}
	if got := CleanFunctionName("foo bar/baz"); got != "foo_bar_baz" {
		t.Fatalf("disallowed→underscore: %q", got)
	}
	long := strings.Repeat("a", 80)
	if got := CleanFunctionName(long); len(got) != 64 {
		t.Fatalf("must cap at 64: len=%d", len(got))
	}
}

func TestCleanJSONSchema_DropsDisallowedFields(t *testing.T) {
	in := map[string]any{
		"$schema":              "x",
		"additionalProperties": false,
		"type":                 []any{"string", "null"},
		"properties": map[string]any{
			"x": map[string]any{"$defs": map[string]any{}, "type": []any{"integer", "null"}},
		},
	}
	CleanJSONSchemaForAntigravity(in)
	for _, k := range []string{"$schema", "additionalProperties"} {
		if _, present := in[k]; present {
			t.Fatalf("expected %q removed", k)
		}
	}
	if got, _ := in["type"].(string); got != "string" {
		t.Fatalf("type collapse: got %v", in["type"])
	}
	innerX := in["properties"].(map[string]any)["x"].(map[string]any)
	if _, present := innerX["$defs"]; present {
		t.Fatal("$defs in nested must be removed too")
	}
	if got, _ := innerX["type"].(string); got != "integer" {
		t.Fatalf("nested type collapse: %v", innerX["type"])
	}
}

func TestMaxTokensForModel_PrefixMatch(t *testing.T) {
	cases := map[string]int{
		"claude-opus-4-7":  32000,
		"claude-haiku-4-5": 4096,
		"gpt-4o-mini":      16384,
		"foo-random":       DefaultMaxTokens,
	}
	for model, want := range cases {
		if got := MaxTokensForModel(model); got != want {
			t.Fatalf("%q want %d got %d", model, want, got)
		}
	}
}

func TestResolveMaxTokens_ExplicitWins(t *testing.T) {
	if got := ResolveMaxTokens(123, "claude-haiku-4-5"); got != 123 {
		t.Fatalf("explicit %d", got)
	}
	if got := ResolveMaxTokens(0, "claude-haiku-4-5"); got != 4096 {
		t.Fatalf("model default %d", got)
	}
}

func TestMergeSystemMessages(t *testing.T) {
	in := []map[string]any{
		{"role": "system", "content": "A"},
		{"role": "user", "content": "u1"},
		{"role": "system", "content": "B"},
		{"role": "assistant", "content": "a1"},
	}
	merged, rest := MergeSystemMessages(in)
	if merged != "A\n\nB" {
		t.Fatalf("merged: %q", merged)
	}
	if len(rest) != 2 {
		t.Fatalf("rest len: %d", len(rest))
	}
}

func TestParseResponsesInput_String(t *testing.T) {
	raw := json.RawMessage(`"hello"`)
	out := ParseResponsesInput(raw)
	if len(out) != 1 || out[0]["content"] != "hello" {
		t.Fatalf("got %v", out)
	}
}

func TestParseResponsesInput_Array(t *testing.T) {
	raw := json.RawMessage(`[{"role":"user","content":"x"},{"role":"assistant","content":[{"type":"text","text":"y"}]}]`)
	out := ParseResponsesInput(raw)
	if len(out) != 2 || out[0]["role"] != "user" || out[1]["role"] != "assistant" {
		t.Fatalf("got %v", out)
	}
	if out[1]["content"] != "y" {
		t.Fatalf("array-of-text-parts flatten: %v", out[1])
	}
}

func TestEncodeResponsesOutput(t *testing.T) {
	out := EncodeResponsesOutput("hi")
	if len(out) != 1 {
		t.Fatal("len")
	}
	msg := out[0]
	if msg["type"] != "message" || msg["role"] != "assistant" {
		t.Fatalf("shape: %v", msg)
	}
}

func TestToolCallID_UniqueAndPrefixed(t *testing.T) {
	id1 := NewToolCallID()
	id2 := NewToolCallID()
	if id1 == id2 {
		t.Fatal("ids must differ")
	}
	if !strings.HasPrefix(id1, "call_") {
		t.Fatalf("missing prefix: %q", id1)
	}
}

func TestAnthropicToolUseToOpenAI_RoundTripShape(t *testing.T) {
	in := map[string]any{
		"id":    "tu_1",
		"name":  "search",
		"input": map[string]any{"q": "rust"},
	}
	out := AnthropicToolUseToOpenAI(in)
	if out["id"] != "tu_1" || out["type"] != "function" {
		t.Fatalf("shape: %v", out)
	}
	fn := out["function"].(map[string]any)
	if fn["name"] != "search" {
		t.Fatalf("name: %v", fn)
	}
	if !strings.Contains(fn["arguments"].(string), `"q":"rust"`) {
		t.Fatalf("args: %v", fn["arguments"])
	}
}

func TestImageHelper_DataURL(t *testing.T) {
	part := map[string]any{
		"image_url": map[string]any{"url": "data:image/png;base64,iVBORw0KGgo="},
	}
	a := OpenAIImageToAnthropic(part)
	src := a["source"].(map[string]any)
	if src["type"] != "base64" || src["media_type"] != "image/png" || src["data"] != "iVBORw0KGgo=" {
		t.Fatalf("data url → anthropic: %v", a)
	}
	g := OpenAIImageToGemini(part)
	inline := g["inline_data"].(map[string]any)
	if inline["mime_type"] != "image/png" {
		t.Fatalf("data url → gemini: %v", g)
	}
}

func TestImageHelper_AnthropicURL(t *testing.T) {
	block := map[string]any{
		"source": map[string]any{
			"type": "url",
			"url":  "https://example.com/x.png",
		},
	}
	o := AnthropicImageToOpenAI(block)
	if o["type"] != "image_url" {
		t.Fatal("type")
	}
}
