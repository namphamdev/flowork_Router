package response

import (
	"testing"
)

func TestBuildOpenAIUsage_NoCacheTokens(t *testing.T) {
	usage := buildOpenAIUsageFromAnthropic(map[string]any{
		"input_tokens":  100.0,
		"output_tokens": 50.0,
	})
	if got := usage["prompt_tokens"].(int64); got != 100 {
		t.Errorf("prompt_tokens: got %d, want 100", got)
	}
	if got := usage["completion_tokens"].(int64); got != 50 {
		t.Errorf("completion_tokens: got %d, want 50", got)
	}
	if got := usage["total_tokens"].(int64); got != 150 {
		t.Errorf("total_tokens: got %d, want 150", got)
	}
	if _, has := usage["prompt_tokens_details"]; has {
		t.Error("prompt_tokens_details should be omitted when no cache tokens")
	}
}

func TestBuildOpenAIUsage_WithCacheRead(t *testing.T) {
	usage := buildOpenAIUsageFromAnthropic(map[string]any{
		"input_tokens":            100.0,
		"output_tokens":           50.0,
		"cache_read_input_tokens": 800.0,
	})
	// prompt_tokens must include cache_read (800 + 100 = 900)
	if got := usage["prompt_tokens"].(int64); got != 900 {
		t.Errorf("prompt_tokens: got %d, want 900", got)
	}
	if got := usage["total_tokens"].(int64); got != 950 {
		t.Errorf("total_tokens: got %d, want 950", got)
	}
	details, ok := usage["prompt_tokens_details"].(map[string]any)
	if !ok {
		t.Fatal("prompt_tokens_details missing")
	}
	if got := details["cached_tokens"].(int64); got != 800 {
		t.Errorf("cached_tokens: got %d, want 800", got)
	}
	if _, has := details["cache_creation_tokens"]; has {
		t.Error("cache_creation_tokens should be omitted when zero")
	}
}

func TestBuildOpenAIUsage_WithBothCacheTokens(t *testing.T) {
	usage := buildOpenAIUsageFromAnthropic(map[string]any{
		"input_tokens":                100.0,
		"output_tokens":               50.0,
		"cache_read_input_tokens":     800.0,
		"cache_creation_input_tokens": 200.0,
	})
	// prompt_tokens = 100 + 800 + 200 = 1100
	if got := usage["prompt_tokens"].(int64); got != 1100 {
		t.Errorf("prompt_tokens: got %d, want 1100", got)
	}
	if got := usage["total_tokens"].(int64); got != 1150 {
		t.Errorf("total_tokens: got %d, want 1150", got)
	}
	details := usage["prompt_tokens_details"].(map[string]any)
	if details["cached_tokens"].(int64) != 800 {
		t.Errorf("cached_tokens wrong: %v", details["cached_tokens"])
	}
	if details["cache_creation_tokens"].(int64) != 200 {
		t.Errorf("cache_creation_tokens wrong: %v", details["cache_creation_tokens"])
	}
}

func TestBuildOpenAIUsage_PreservesNativeAnthropicFields(t *testing.T) {
	usage := buildOpenAIUsageFromAnthropic(map[string]any{
		"input_tokens":  100.0,
		"output_tokens": 50.0,
	})
	if got := usage["input_tokens"].(int64); got != 100 {
		t.Errorf("input_tokens passthrough lost: %v", got)
	}
	if got := usage["output_tokens"].(int64); got != 50 {
		t.Errorf("output_tokens passthrough lost: %v", got)
	}
}

func TestBuildOpenAIUsage_NilInputSafelyHandled(t *testing.T) {
	usage := buildOpenAIUsageFromAnthropic(nil)
	if got := usage["prompt_tokens"].(int64); got != 0 {
		t.Errorf("nil input should yield zero prompt_tokens, got %d", got)
	}
}

func TestClaudeToOpenAI_EmitsCacheBreakdown(t *testing.T) {
	out := ClaudeToOpenAI(map[string]any{
		"id":          "msg_x",
		"model":       "claude-sonnet-4-5",
		"stop_reason": "end_turn",
		"content": []any{
			map[string]any{"type": "text", "text": "hello"},
		},
		"usage": map[string]any{
			"input_tokens":            50.0,
			"output_tokens":           20.0,
			"cache_read_input_tokens": 500.0,
		},
	})
	usage := out["usage"].(map[string]any)
	if got := usage["prompt_tokens"].(int64); got != 550 {
		t.Fatalf("end-to-end translator missing cache_read: prompt_tokens=%d", got)
	}
	details := usage["prompt_tokens_details"].(map[string]any)
	if details["cached_tokens"].(int64) != 500 {
		t.Fatalf("end-to-end translator missing cached_tokens: %v", details)
	}
}
