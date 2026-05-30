// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package response

import (
	"encoding/json"
	"strings"
	"testing"
)

// parseNDJSON splits the buffer into one map per non-blank line.
func parseNDJSON(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("ndjson parse failed: %v\nline=%q", err, line)
		}
		out = append(out, m)
	}
	return out
}

func TestOllamaTransform_PlainTextChunks(t *testing.T) {
	src := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":", world"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
`
	rows := parseNDJSON(t, TransformOpenAISSEToOllamaBytes([]byte(src), "llama-3"))
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (2 deltas + 1 done), got %d", len(rows))
	}
	if rows[0]["model"] != "llama-3" {
		t.Errorf("model leaked: %v", rows[0]["model"])
	}
	if rows[0]["done"] != false {
		t.Errorf("first row done flag wrong: %v", rows[0]["done"])
	}
	// last row done=true.
	if rows[2]["done"] != true {
		t.Errorf("last row should be done=true: %+v", rows[2])
	}
}

func TestOllamaTransform_ToolCallsAggregated(t *testing.T) {
	src := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"get_w"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc\":"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"NYC\"}"}}]}}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
`
	rows := parseNDJSON(t, TransformOpenAISSEToOllamaBytes([]byte(src), "qwen"))
	// last row carries the aggregated tool call.
	last := rows[len(rows)-1]
	if last["done"] != true {
		t.Fatalf("last row must be done=true: %+v", last)
	}
	msg, _ := last["message"].(map[string]any)
	tcs, ok := msg["tool_calls"].([]any)
	if !ok || len(tcs) != 1 {
		t.Fatalf("expected 1 tool_call, got %v", tcs)
	}
	tc := tcs[0].(map[string]any)
	fn := tc["function"].(map[string]any)
	if fn["name"] != "get_w" {
		t.Errorf("tool name wrong: %v", fn["name"])
	}
	args := fn["arguments"].(map[string]any)
	if args["loc"] != "NYC" {
		t.Errorf("tool arguments not parsed: %v", args)
	}
}

func TestOllamaTransform_MissingDONESentinelStillCloses(t *testing.T) {
	// Upstream cut the stream mid-way without sending [DONE]. We still
	// emit a final done=true row so consumers see clean termination.
	src := `data: {"choices":[{"delta":{"content":"hi"}}]}
`
	rows := parseNDJSON(t, TransformOpenAISSEToOllamaBytes([]byte(src), "x"))
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (delta + auto-done), got %d", len(rows))
	}
	if rows[1]["done"] != true {
		t.Errorf("auto-done not emitted: %+v", rows[1])
	}
}

func TestOllamaTransform_MalformedDataLinesSkipped(t *testing.T) {
	src := `data: not-json
data: {"choices":[{"delta":{"content":"good"}}]}
data: {"":bogus}
data: [DONE]
`
	rows := parseNDJSON(t, TransformOpenAISSEToOllamaBytes([]byte(src), "m"))
	// Expect: good delta + done = 2 rows. Malformed lines must NOT crash.
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestOllamaTransform_NoContentNoOpExceptDone(t *testing.T) {
	src := `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
`
	rows := parseNDJSON(t, TransformOpenAISSEToOllamaBytes([]byte(src), "x"))
	// Only the done row.
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 done row, got %d", len(rows))
	}
	if rows[0]["done"] != true {
		t.Errorf("final row not marked done: %+v", rows[0])
	}
}

func TestOllamaTransform_ToolCallsOrderedByIndex(t *testing.T) {
	src := `data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"b","function":{"name":"second","arguments":"{}"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"first","arguments":"{}"}}]}}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
`
	rows := parseNDJSON(t, TransformOpenAISSEToOllamaBytes([]byte(src), "m"))
	last := rows[len(rows)-1]
	tcs := last["message"].(map[string]any)["tool_calls"].([]any)
	if len(tcs) != 2 {
		t.Fatalf("expected 2 tool_calls, got %d", len(tcs))
	}
	first := tcs[0].(map[string]any)["function"].(map[string]any)
	if first["name"] != "first" {
		t.Errorf("index 0 should come first, got %v", first["name"])
	}
}
