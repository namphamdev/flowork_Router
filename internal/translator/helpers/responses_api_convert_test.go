package helpers

import (
	"encoding/json"
	"testing"
)

func TestNormalizeResponsesInput_PlainString(t *testing.T) {
	out := NormalizeResponsesInput(json.RawMessage(`"hello"`))
	if len(out) != 1 {
		t.Fatalf("expected 1 item, got %d", len(out))
	}
	if out[0]["role"] != "user" {
		t.Errorf("role: %v", out[0]["role"])
	}
	parts := out[0]["content"].([]map[string]any)
	if parts[0]["text"] != "hello" || parts[0]["type"] != "input_text" {
		t.Errorf("content wrong: %+v", parts[0])
	}
}

func TestNormalizeResponsesInput_EmptyStringInjectsPlaceholder(t *testing.T) {
	out := NormalizeResponsesInput(json.RawMessage(`""`))
	if len(out) != 1 {
		t.Fatalf("placeholder missing")
	}
	parts := out[0]["content"].([]map[string]any)
	if parts[0]["text"] != "..." {
		t.Errorf("placeholder text: %v", parts[0]["text"])
	}
}

func TestNormalizeResponsesInput_EmptyArrayInjectsPlaceholder(t *testing.T) {
	out := NormalizeResponsesInput(json.RawMessage(`[]`))
	if len(out) != 1 || out[0]["role"] != "user" {
		t.Fatalf("placeholder for empty array missing")
	}
}

func TestNormalizeResponsesInput_ArrayPassthrough(t *testing.T) {
	src := `[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]`
	out := NormalizeResponsesInput(json.RawMessage(src))
	if len(out) != 1 {
		t.Fatalf("array passthrough lost item: %v", out)
	}
}

func TestConvertResponsesAPIFormat_PlainMessage(t *testing.T) {
	body := map[string]any{
		"input":        "what is 2+2?",
		"instructions": "be terse",
	}
	got := ConvertResponsesAPIFormat(body)
	msgs := got["messages"].([]map[string]any)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 msgs (system+user), got %d", len(msgs))
	}
	if msgs[0]["role"] != "system" || msgs[0]["content"] != "be terse" {
		t.Errorf("system msg wrong: %+v", msgs[0])
	}
	if msgs[1]["role"] != "user" {
		t.Errorf("user role wrong: %v", msgs[1]["role"])
	}
	// input/instructions/store should be stripped
	for _, k := range []string{"input", "instructions", "include", "prompt_cache_key", "store", "reasoning"} {
		if _, has := got[k]; has {
			t.Errorf("%s should be stripped", k)
		}
	}
}

func TestConvertResponsesAPIFormat_FunctionCallGroups(t *testing.T) {
	body := map[string]any{
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "what's the weather?"},
			}},
			map[string]any{"type": "function_call", "call_id": "fc_1", "name": "get_weather", "arguments": `{"loc":"NYC"}`},
			map[string]any{"type": "function_call", "call_id": "fc_2", "name": "get_time", "arguments": `{}`},
			map[string]any{"type": "function_call_output", "call_id": "fc_1", "output": "75°F sunny"},
			map[string]any{"type": "function_call_output", "call_id": "fc_2", "output": "noon"},
		},
	}
	got := ConvertResponsesAPIFormat(body)
	msgs := got["messages"].([]map[string]any)
	// Expected: user, assistant (2 tool_calls), tool(fc_1), tool(fc_2) = 4
	if len(msgs) != 4 {
		t.Fatalf("expected 4 msgs, got %d: %+v", len(msgs), msgs)
	}
	if msgs[1]["role"] != "assistant" {
		t.Errorf("msg[1] should be assistant, got %v", msgs[1]["role"])
	}
	tcs := msgs[1]["tool_calls"].([]map[string]any)
	if len(tcs) != 2 {
		t.Fatalf("expected 2 tool_calls grouped, got %d", len(tcs))
	}
	if tcs[0]["id"] != "fc_1" || tcs[1]["id"] != "fc_2" {
		t.Errorf("tool_call ids: %v / %v", tcs[0]["id"], tcs[1]["id"])
	}
	if msgs[2]["role"] != "tool" || msgs[2]["tool_call_id"] != "fc_1" {
		t.Errorf("tool result[0] wrong: %+v", msgs[2])
	}
	if msgs[3]["role"] != "tool" || msgs[3]["tool_call_id"] != "fc_2" {
		t.Errorf("tool result[1] wrong: %+v", msgs[3])
	}
}

func TestConvertResponsesAPIFormat_DropsNamelessFunctionCalls(t *testing.T) {
	body := map[string]any{
		"input": []any{
			map[string]any{"type": "function_call", "call_id": "x", "name": "", "arguments": "{}"},
			map[string]any{"type": "function_call", "call_id": "y", "name": "ok", "arguments": "{}"},
		},
	}
	got := ConvertResponsesAPIFormat(body)
	msgs := got["messages"].([]map[string]any)
	if len(msgs) != 1 {
		t.Fatalf("expected only the good call, got %d msgs", len(msgs))
	}
	tcs := msgs[0]["tool_calls"].([]map[string]any)
	if len(tcs) != 1 || tcs[0]["id"] != "y" {
		t.Fatalf("nameless call not dropped: %+v", tcs)
	}
}

func TestConvertResponsesAPIFormat_DropsReasoningItems(t *testing.T) {
	body := map[string]any{
		"input": []any{
			map[string]any{"type": "reasoning", "text": "thinking…"},
			map[string]any{"type": "message", "role": "user", "content": "hi"},
		},
	}
	got := ConvertResponsesAPIFormat(body)
	msgs := got["messages"].([]map[string]any)
	if len(msgs) != 1 {
		t.Fatalf("reasoning should be dropped, got %d msgs", len(msgs))
	}
	if msgs[0]["role"] != "user" {
		t.Errorf("only user msg should survive, got %v", msgs[0]["role"])
	}
}

func TestConvertResponsesAPIFormat_InputImage(t *testing.T) {
	body := map[string]any{
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": []any{
				map[string]any{"type": "input_image", "image_url": "https://example.com/cat.png", "detail": "high"},
			}},
		},
	}
	got := ConvertResponsesAPIFormat(body)
	msgs := got["messages"].([]map[string]any)
	parts := msgs[0]["content"].([]map[string]any)
	if parts[0]["type"] != "image_url" {
		t.Fatalf("input_image not normalised: %+v", parts[0])
	}
	img := parts[0]["image_url"].(map[string]any)
	if img["url"] != "https://example.com/cat.png" || img["detail"] != "high" {
		t.Errorf("image fields wrong: %+v", img)
	}
}

func TestConvertResponsesAPIFormat_NoInputPassesThrough(t *testing.T) {
	body := map[string]any{"messages": []any{}, "model": "x"}
	got := ConvertResponsesAPIFormat(body)
	if _, has := got["messages"]; !has {
		t.Fatal("no-input path should leave body unchanged")
	}
}
