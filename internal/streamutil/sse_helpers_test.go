package streamutil

import (
	"strings"
	"testing"
)

// ── ParseSSELine ──────────────────────────────────────────────────────

func TestParseSSELine_EmptyReturnsNil(t *testing.T) {
	if ParseSSELine("", FormatOpenAI) != nil {
		t.Fatal("empty line should return nil")
	}
}

func TestParseSSELine_CommentLineReturnsNil(t *testing.T) {
	if ParseSSELine(": ping", FormatOpenAI) != nil {
		t.Fatal("comment lines (start with ':') must return nil")
	}
}

func TestParseSSELine_DataPrefixObject(t *testing.T) {
	got := ParseSSELine(`data: {"id":"x","choices":[]}`, FormatOpenAI)
	if got == nil || got.Object == nil {
		t.Fatal("data line should decode into Object")
	}
	if got.Object["id"] != "x" {
		t.Errorf("id lost: %v", got.Object["id"])
	}
	if got.Done {
		t.Error("Done flag should be false for a non-DONE line")
	}
}

func TestParseSSELine_DoneSentinel(t *testing.T) {
	got := ParseSSELine("data: [DONE]", FormatClaude)
	if got == nil || !got.Done {
		t.Fatalf("[DONE] sentinel must set Done=true, got %+v", got)
	}
}

func TestParseSSELine_MalformedJSONReturnsNil(t *testing.T) {
	if ParseSSELine(`data: {not-json`, FormatOpenAI) != nil {
		t.Fatal("malformed JSON must return nil, not panic")
	}
}

func TestParseSSELine_OllamaNDJSON(t *testing.T) {
	got := ParseSSELine(`{"model":"x","done":false}`, FormatOllama)
	if got == nil || got.Object == nil {
		t.Fatal("ollama NDJSON should decode")
	}
	if got.Object["model"] != "x" {
		t.Errorf("model lost: %v", got.Object["model"])
	}
}

func TestParseSSELine_OllamaIgnoresDataPrefix(t *testing.T) {
	// Ollama format MUST not match "data:" lines (those belong to SSE
	// formats) — returning nil keeps the contract clean.
	if ParseSSELine(`data: {"x":1}`, FormatOllama) != nil {
		t.Fatal("ollama parser must not match data: lines")
	}
}

func TestParseSSELine_OllamaNonJSONLineReturnsNil(t *testing.T) {
	if ParseSSELine(`not-a-json-line`, FormatOllama) != nil {
		t.Fatal("ollama parser must reject non-JSON lines")
	}
}

// ── HasValuableContent ────────────────────────────────────────────────

func TestHasValuableContent_OpenAIContentDelta(t *testing.T) {
	chunk := map[string]any{
		"choices": []any{
			map[string]any{"delta": map[string]any{"content": "hi"}},
		},
	}
	if !HasValuableContent(chunk, FormatOpenAI) {
		t.Fatal("content delta is valuable")
	}
}

func TestHasValuableContent_OpenAIReasoningOnly(t *testing.T) {
	chunk := map[string]any{
		"choices": []any{
			map[string]any{"delta": map[string]any{"reasoning_content": "thinking…"}},
		},
	}
	if !HasValuableContent(chunk, FormatOpenAI) {
		t.Fatal("reasoning_content delta is valuable")
	}
}

func TestHasValuableContent_OpenAIToolCalls(t *testing.T) {
	chunk := map[string]any{
		"choices": []any{
			map[string]any{"delta": map[string]any{"tool_calls": []any{map[string]any{"id": "c"}}}},
		},
	}
	if !HasValuableContent(chunk, FormatOpenAI) {
		t.Fatal("tool_calls in delta is valuable")
	}
}

func TestHasValuableContent_OpenAIFinishReason(t *testing.T) {
	chunk := map[string]any{
		"choices": []any{
			map[string]any{"delta": map[string]any{}, "finish_reason": "stop"},
		},
	}
	if !HasValuableContent(chunk, FormatOpenAI) {
		t.Fatal("finish_reason marker is valuable")
	}
}

func TestHasValuableContent_OpenAIEmptyDeltaIsNotValuable(t *testing.T) {
	chunk := map[string]any{
		"choices": []any{
			map[string]any{"delta": map[string]any{}},
		},
	}
	if HasValuableContent(chunk, FormatOpenAI) {
		t.Fatal("empty delta should be filtered out")
	}
}

func TestHasValuableContent_OpenAIRoleAnnouncement(t *testing.T) {
	chunk := map[string]any{
		"choices": []any{
			map[string]any{"delta": map[string]any{"role": "assistant"}},
		},
	}
	if !HasValuableContent(chunk, FormatOpenAI) {
		t.Fatal("role announcement (first chunk) is valuable")
	}
}

func TestHasValuableContent_ClaudeEmptyBlockDeltaFilteredOut(t *testing.T) {
	chunk := map[string]any{
		"type":  "content_block_delta",
		"delta": map[string]any{},
	}
	if HasValuableContent(chunk, FormatClaude) {
		t.Fatal("empty content_block_delta is heartbeat noise")
	}
}

func TestHasValuableContent_ClaudeTextDelta(t *testing.T) {
	chunk := map[string]any{
		"type":  "content_block_delta",
		"delta": map[string]any{"text": "hi"},
	}
	if !HasValuableContent(chunk, FormatClaude) {
		t.Fatal("text delta is valuable")
	}
}

func TestHasValuableContent_ClaudeNonDeltaAlwaysKept(t *testing.T) {
	chunk := map[string]any{"type": "message_start"}
	if !HasValuableContent(chunk, FormatClaude) {
		t.Fatal("non-delta Claude events must be kept")
	}
}

func TestHasValuableContent_UnknownFormatDefaultsToKeep(t *testing.T) {
	if !HasValuableContent(map[string]any{}, "weird-format") {
		t.Fatal("unknown format must default to keep-everything")
	}
}

func TestHasValuableContent_NilChunkIsFiltered(t *testing.T) {
	if HasValuableContent(nil, FormatOpenAI) {
		t.Fatal("nil chunk is never valuable")
	}
}

// ── FixInvalidID ──────────────────────────────────────────────────────

func TestFixInvalidID_GenericChatPlaceholder(t *testing.T) {
	chunk := map[string]any{"id": "chat"}
	if !FixInvalidID(chunk) {
		t.Fatal("placeholder 'chat' should be rewritten")
	}
	if !strings.HasPrefix(chunk["id"].(string), "chatcmpl-") {
		t.Errorf("expected chatcmpl- prefix, got %v", chunk["id"])
	}
}

func TestFixInvalidID_ShortIDRewritten(t *testing.T) {
	chunk := map[string]any{"id": "abc"} // len 3 < 8
	if !FixInvalidID(chunk) {
		t.Fatal("short id should be rewritten")
	}
}

func TestFixInvalidID_ValidIDLeftAlone(t *testing.T) {
	chunk := map[string]any{"id": "chatcmpl-1234567890abcdef"}
	if FixInvalidID(chunk) {
		t.Fatal("valid id must not be touched")
	}
	if chunk["id"] != "chatcmpl-1234567890abcdef" {
		t.Errorf("id mutated: %v", chunk["id"])
	}
}

func TestFixInvalidID_MissingIDLeftAlone(t *testing.T) {
	chunk := map[string]any{}
	if FixInvalidID(chunk) {
		t.Fatal("missing id is a different concern; helper should not rewrite")
	}
}

func TestFixInvalidID_UsesRequestIDFallback(t *testing.T) {
	chunk := map[string]any{
		"id": "chat",
		"extend_fields": map[string]any{
			"requestId": "req-abc-123",
		},
	}
	FixInvalidID(chunk)
	if chunk["id"] != "chatcmpl-req-abc-123" {
		t.Errorf("requestId fallback not used: %v", chunk["id"])
	}
}

func TestFixInvalidID_UsesTraceIDFallback(t *testing.T) {
	chunk := map[string]any{
		"id": "chat",
		"extend_fields": map[string]any{
			"traceId": "tr-xyz",
		},
	}
	FixInvalidID(chunk)
	if chunk["id"] != "chatcmpl-tr-xyz" {
		t.Errorf("traceId fallback not used: %v", chunk["id"])
	}
}

func TestFixInvalidID_RequestIDWinsOverTraceID(t *testing.T) {
	chunk := map[string]any{
		"id": "chat",
		"extend_fields": map[string]any{
			"requestId": "REQ",
			"traceId":   "TRACE",
		},
	}
	FixInvalidID(chunk)
	if chunk["id"] != "chatcmpl-REQ" {
		t.Errorf("requestId should win over traceId, got %v", chunk["id"])
	}
}

func TestFixInvalidID_TimestampFallbackBase36(t *testing.T) {
	chunk := map[string]any{"id": "chat"}
	FixInvalidID(chunk)
	got := chunk["id"].(string)
	if !strings.HasPrefix(got, "chatcmpl-") {
		t.Fatalf("prefix wrong: %s", got)
	}
	suffix := got[len("chatcmpl-"):]
	if len(suffix) < 6 {
		t.Errorf("base36 timestamp too short: %s", suffix)
	}
	// All chars must be base36 (digits + lowercase letters).
	for _, c := range suffix {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
			t.Errorf("non-base36 char %q in fallback id %q", c, got)
		}
	}
}

func TestFixInvalidID_NilChunkIsSafe(t *testing.T) {
	if FixInvalidID(nil) {
		t.Fatal("nil chunk must return false without panic")
	}
}
