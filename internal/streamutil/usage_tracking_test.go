package streamutil

import (
	"testing"
)

// ── AddBufferToUsage ─────────────────────────────────────────────────

func TestAddBufferToUsage_OpenAIShape(t *testing.T) {
	got := AddBufferToUsage(map[string]any{
		"prompt_tokens":     float64(100),
		"completion_tokens": float64(50),
		"total_tokens":      float64(150),
	})
	if got["prompt_tokens"].(float64) != 100+BufferTokens {
		t.Errorf("prompt_tokens not buffered: %v", got["prompt_tokens"])
	}
	if got["total_tokens"].(float64) != 150+BufferTokens {
		t.Errorf("total_tokens not buffered: %v", got["total_tokens"])
	}
}

func TestAddBufferToUsage_ClaudeShape(t *testing.T) {
	got := AddBufferToUsage(map[string]any{
		"input_tokens":  float64(100),
		"output_tokens": float64(50),
	})
	if got["input_tokens"].(float64) != 100+BufferTokens {
		t.Errorf("input_tokens not buffered: %v", got["input_tokens"])
	}
	// Claude shape doesn't carry total_tokens — should NOT derive one
	// (the buffer logic only derives when BOTH prompt + completion exist).
	if _, has := got["total_tokens"]; has {
		t.Errorf("total_tokens should not be invented for Claude shape: %v", got["total_tokens"])
	}
}

func TestAddBufferToUsage_DerivesMissingTotal(t *testing.T) {
	got := AddBufferToUsage(map[string]any{
		"prompt_tokens":     float64(100),
		"completion_tokens": float64(50),
	})
	// After buffering: prompt becomes 2100; total derived from buffered prompt + completion.
	want := float64(100+BufferTokens) + float64(50)
	if got["total_tokens"].(float64) != want {
		t.Errorf("derived total wrong: got %v want %v", got["total_tokens"], want)
	}
}

func TestAddBufferToUsage_NilInputUnchanged(t *testing.T) {
	if AddBufferToUsage(nil) != nil {
		t.Fatal("nil should pass through")
	}
}

// ── NormalizeUsage ────────────────────────────────────────────────────

func TestNormalizeUsage_CoercesNumericTypes(t *testing.T) {
	got := NormalizeUsage(map[string]any{
		"prompt_tokens":     int(100),  // int
		"completion_tokens": int64(50), // int64
		"total_tokens":      float64(150),
	})
	if got["prompt_tokens"].(float64) != 100 {
		t.Errorf("int not coerced: %T %v", got["prompt_tokens"], got["prompt_tokens"])
	}
	if got["completion_tokens"].(float64) != 50 {
		t.Errorf("int64 not coerced: %T %v", got["completion_tokens"], got["completion_tokens"])
	}
}

func TestNormalizeUsage_PreservesDetailsObjects(t *testing.T) {
	got := NormalizeUsage(map[string]any{
		"prompt_tokens": float64(100),
		"prompt_tokens_details": map[string]any{
			"cached_tokens": float64(50),
		},
	})
	d, ok := got["prompt_tokens_details"].(map[string]any)
	if !ok {
		t.Fatal("details object lost")
	}
	if d["cached_tokens"].(float64) != 50 {
		t.Errorf("nested details mutated: %v", d)
	}
}

func TestNormalizeUsage_DropsNaNAndStrings(t *testing.T) {
	got := NormalizeUsage(map[string]any{
		"prompt_tokens": "not a number",
		"total_tokens":  float64(150),
	})
	if _, has := got["prompt_tokens"]; has {
		t.Error("string value should be dropped")
	}
	if got["total_tokens"].(float64) != 150 {
		t.Error("valid value lost")
	}
}

func TestNormalizeUsage_EmptyInputReturnsNil(t *testing.T) {
	if NormalizeUsage(nil) != nil {
		t.Fatal("nil → nil")
	}
	if NormalizeUsage(map[string]any{}) != nil {
		t.Fatal("empty map should return nil (no usable fields)")
	}
}

// ── FilterUsageForFormat ──────────────────────────────────────────────

func TestFilterUsageForFormat_OpenAIKeepsOpenAIKeys(t *testing.T) {
	got := FilterUsageForFormat(map[string]any{
		"prompt_tokens":     float64(10),
		"completion_tokens": float64(5),
		"input_tokens":      float64(99), // Claude key — should be filtered
		"random_field":      "leak",
	}, FormatOpenAI)
	if got["prompt_tokens"].(float64) != 10 {
		t.Error("prompt_tokens dropped")
	}
	if _, has := got["input_tokens"]; has {
		t.Error("Claude key leaked into OpenAI usage")
	}
	if _, has := got["random_field"]; has {
		t.Error("unknown field leaked")
	}
}

func TestFilterUsageForFormat_ClaudeKeepsClaudeKeys(t *testing.T) {
	got := FilterUsageForFormat(map[string]any{
		"input_tokens":            float64(10),
		"output_tokens":           float64(5),
		"cache_read_input_tokens": float64(2),
		"prompt_tokens":           float64(99),
	}, FormatClaude)
	if got["input_tokens"].(float64) != 10 || got["cache_read_input_tokens"].(float64) != 2 {
		t.Errorf("claude keys lost: %+v", got)
	}
	if _, has := got["prompt_tokens"]; has {
		t.Error("OpenAI key leaked into Claude usage")
	}
}

func TestFilterUsageForFormat_GeminiKeepsGeminiKeys(t *testing.T) {
	got := FilterUsageForFormat(map[string]any{
		"promptTokenCount":     float64(10),
		"candidatesTokenCount": float64(5),
		"totalTokenCount":      float64(15),
		"prompt_tokens":        float64(99),
	}, FormatGemini)
	if got["promptTokenCount"].(float64) != 10 {
		t.Errorf("gemini key lost: %v", got["promptTokenCount"])
	}
	if _, has := got["prompt_tokens"]; has {
		t.Error("OpenAI key leaked into Gemini usage")
	}
}

func TestFilterUsageForFormat_UnknownFormatPassesThrough(t *testing.T) {
	src := map[string]any{"anything": float64(1)}
	got := FilterUsageForFormat(src, "weird-format")
	if got["anything"].(float64) != 1 {
		t.Error("unknown format should leave usage intact")
	}
}

func TestFilterUsageForFormat_NilReturnsNil(t *testing.T) {
	if FilterUsageForFormat(nil, FormatOpenAI) != nil {
		t.Fatal("nil → nil")
	}
}

// ── EstimateInputTokens / EstimateOutputTokens / EstimateUsage ───────

func TestEstimateInputTokens_ZeroForEmpty(t *testing.T) {
	if EstimateInputTokens(nil) != 0 {
		t.Error("nil body should be zero")
	}
}

func TestEstimateInputTokens_RoughlyOneTokenPer4Chars(t *testing.T) {
	body := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": "1234567890"},
	}}
	// JSON serialised form is ~50 chars → ~13 tokens. Allow a wide window.
	got := EstimateInputTokens(body)
	if got < 5 || got > 50 {
		t.Errorf("rough estimate out of expected range: %d", got)
	}
}

func TestEstimateOutputTokens(t *testing.T) {
	if EstimateOutputTokens(0) != 0 {
		t.Error("zero content → zero tokens")
	}
	if EstimateOutputTokens(1) != 1 {
		t.Error("tiny content should clamp to at least 1 token")
	}
	if EstimateOutputTokens(40) != 10 {
		t.Errorf("40 chars / 4 → 10, got %d", EstimateOutputTokens(40))
	}
}

func TestEstimateUsage_OpenAIShape(t *testing.T) {
	got := EstimateUsage(map[string]any{"messages": []any{"hi"}}, 40, FormatOpenAI)
	if _, has := got["prompt_tokens"]; !has {
		t.Error("OpenAI estimate missing prompt_tokens")
	}
	if got["estimated"] != true {
		t.Error("estimated=true flag missing")
	}
}

func TestEstimateUsage_ClaudeShape(t *testing.T) {
	got := EstimateUsage(map[string]any{"messages": []any{"hi"}}, 40, FormatClaude)
	if _, has := got["input_tokens"]; !has {
		t.Error("Claude estimate missing input_tokens")
	}
	if _, has := got["prompt_tokens"]; has {
		t.Error("Claude estimate leaked prompt_tokens")
	}
}

func TestEstimateUsage_GeminiShape(t *testing.T) {
	got := EstimateUsage(map[string]any{"messages": []any{"hi"}}, 40, FormatGemini)
	if _, has := got["promptTokenCount"]; !has {
		t.Error("Gemini estimate missing promptTokenCount")
	}
	if _, has := got["totalTokenCount"]; !has {
		t.Error("Gemini estimate missing totalTokenCount")
	}
}

// ── HasValidUsage ─────────────────────────────────────────────────────

func TestHasValidUsage_OpenAIPositiveCounter(t *testing.T) {
	if !HasValidUsage(map[string]any{"prompt_tokens": float64(1)}) {
		t.Fatal("non-zero prompt_tokens is valid")
	}
}

func TestHasValidUsage_ClaudePositiveCounter(t *testing.T) {
	if !HasValidUsage(map[string]any{"input_tokens": float64(1)}) {
		t.Fatal("non-zero input_tokens is valid")
	}
}

func TestHasValidUsage_AllZeroIsNotValid(t *testing.T) {
	if HasValidUsage(map[string]any{
		"prompt_tokens":     float64(0),
		"completion_tokens": float64(0),
		"total_tokens":      float64(0),
	}) {
		t.Fatal("zeroed counters should not count as valid usage")
	}
}

func TestHasValidUsage_NilReturnsFalse(t *testing.T) {
	if HasValidUsage(nil) {
		t.Fatal("nil should be false")
	}
}
