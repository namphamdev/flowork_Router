// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package router

import (
	"strings"
	"testing"
)

func TestParseSSEToOpenAIResponse_EmptyInput(t *testing.T) {
	if got := ParseSSEToOpenAIResponse(nil, "x"); got != nil {
		t.Fatalf("nil input must return nil, got %v", got)
	}
	if got := ParseSSEToOpenAIResponse([]byte(""), "x"); got != nil {
		t.Fatalf("empty input must return nil, got %v", got)
	}
}

func TestParseSSEToOpenAIResponse_NoDataLines(t *testing.T) {
	body := "event: ping\n\nevent: noop\n\n"
	if got := ParseSSEToOpenAIResponse([]byte(body), "x"); got != nil {
		t.Fatalf("no data lines must return nil, got %v", got)
	}
}

func TestParseSSEToOpenAIResponse_ConcatenatesContent(t *testing.T) {
	body := `data: {"id":"x1","model":"m","choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":", "}}]}
data: {"choices":[{"delta":{"content":"world"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
`
	res := ParseSSEToOpenAIResponse([]byte(body), "fallback-model")
	if res == nil {
		t.Fatal("expected aggregated result")
	}
	if res["model"] != "m" {
		t.Errorf("model mismatch: %v", res["model"])
	}
	choices := res["choices"].([]map[string]any)
	msg := choices[0]["message"].(map[string]any)
	if msg["content"] != "Hello, world" {
		t.Errorf("content concat wrong: %q", msg["content"])
	}
	if choices[0]["finish_reason"] != "stop" {
		t.Errorf("finish_reason lost: %v", choices[0]["finish_reason"])
	}
}

func TestParseSSEToOpenAIResponse_AccumulatesToolCalls(t *testing.T) {
	body := `data: {"id":"x","model":"m","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"get_w"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":\"NYC\"}"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_2","function":{"name":"get_t","arguments":"{}"}}]}}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
`
	res := ParseSSEToOpenAIResponse([]byte(body), "fallback")
	if res == nil {
		t.Fatal("expected result")
	}
	msg := res["choices"].([]map[string]any)[0]["message"].(map[string]any)
	tcs, _ := msg["tool_calls"].([]map[string]any)
	if len(tcs) != 2 {
		t.Fatalf("expected 2 tool_calls, got %d", len(tcs))
	}
	first := tcs[0]
	if first["id"] != "call_1" {
		t.Errorf("call[0].id = %v", first["id"])
	}
	fn0 := first["function"].(map[string]any)
	if fn0["name"] != "get_w" {
		t.Errorf("call[0].name = %v", fn0["name"])
	}
	if args, _ := fn0["arguments"].(string); !strings.Contains(args, "NYC") {
		t.Errorf("call[0].arguments not concatenated: %q", args)
	}
	if tcs[1]["id"] != "call_2" {
		t.Errorf("call[1].id = %v", tcs[1]["id"])
	}
}

func TestParseSSEToOpenAIResponse_SkipsMalformedLines(t *testing.T) {
	body := `data: not-json
data: {"id":"x","choices":[{"delta":{"content":"hi"}}]}
data: {"":}
`
	res := ParseSSEToOpenAIResponse([]byte(body), "m")
	if res == nil {
		t.Fatal("malformed should be skipped, not abort")
	}
	msg := res["choices"].([]map[string]any)[0]["message"].(map[string]any)
	if msg["content"] != "hi" {
		t.Errorf("good chunk lost: %v", msg["content"])
	}
}

func TestParseSSEToOpenAIResponse_PreservesUsage(t *testing.T) {
	body := `data: {"id":"x","choices":[{"delta":{"content":"hi"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}
data: [DONE]
`
	res := ParseSSEToOpenAIResponse([]byte(body), "m")
	usage, ok := res["usage"].(map[string]any)
	if !ok {
		t.Fatal("usage missing")
	}
	if int(usage["total_tokens"].(float64)) != 12 {
		t.Errorf("total_tokens lost: %v", usage["total_tokens"])
	}
}

func TestParseSSEToOpenAIResponse_FallbackModel(t *testing.T) {
	body := `data: {"id":"x","choices":[{"delta":{"content":"hi"}}]}
data: [DONE]
`
	res := ParseSSEToOpenAIResponse([]byte(body), "fallback-claude")
	if res["model"] != "fallback-claude" {
		t.Fatalf("fallback model not applied: %v", res["model"])
	}
}

func TestParseSSEToOpenAIResponse_ReasoningContentPreserved(t *testing.T) {
	body := `data: {"id":"x","choices":[{"delta":{"reasoning_content":"thinking…"}}]}
data: {"choices":[{"delta":{"content":"answer"}}]}
data: [DONE]
`
	res := ParseSSEToOpenAIResponse([]byte(body), "m")
	msg := res["choices"].([]map[string]any)[0]["message"].(map[string]any)
	if r, _ := msg["reasoning_content"].(string); r != "thinking…" {
		t.Errorf("reasoning lost: %q", r)
	}
}

func TestParseSSEToOpenAIResponse_NullContentWhenOnlyToolCalls(t *testing.T) {
	body := `data: {"id":"x","model":"m","choices":[{"delta":{"tool_calls":[{"index":0,"id":"c","function":{"name":"f","arguments":"{}"}}]}}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
`
	res := ParseSSEToOpenAIResponse([]byte(body), "m")
	msg := res["choices"].([]map[string]any)[0]["message"].(map[string]any)
	if msg["content"] != nil {
		t.Errorf("content should be nil when only tool_calls, got %v", msg["content"])
	}
}
