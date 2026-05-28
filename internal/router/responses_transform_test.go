package router

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// captureEvents parses the SSE buffer into ordered (event, data) pairs.
type sseEvent struct {
	Event string
	Data  map[string]any
}

func parseSSEEvents(t *testing.T, buf string) []sseEvent {
	t.Helper()
	var out []sseEvent
	for _, block := range strings.Split(buf, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var ev sseEvent
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev.Data)
			}
		}
		if ev.Event != "" {
			out = append(out, ev)
		}
	}
	return out
}

func TestResponsesSSE_EmitsCreatedFirst(t *testing.T) {
	var buf bytes.Buffer
	r := NewResponsesSSEWriter(&buf, nil, "claude-sonnet")
	r.Close()
	events := parseSSEEvents(t, buf.String())
	if len(events) < 2 || events[0].Event != "response.created" {
		t.Fatalf("first event must be response.created, got %v", events)
	}
	last := events[len(events)-1]
	if last.Event != "response.completed" {
		t.Fatalf("last event must be response.completed, got %s", last.Event)
	}
}

func TestResponsesSSE_TextDeltaSequence(t *testing.T) {
	var buf bytes.Buffer
	r := NewResponsesSSEWriter(&buf, nil, "m")
	for _, chunk := range []string{"Hello", ", ", "world"} {
		r.Feed(map[string]any{
			"choices": []any{
				map[string]any{
					"delta": map[string]any{"content": chunk},
				},
			},
		})
	}
	r.Close()

	events := parseSSEEvents(t, buf.String())
	var seen []string
	for _, e := range events {
		seen = append(seen, e.Event)
	}
	want := []string{
		"response.created",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	if len(seen) != len(want) {
		t.Fatalf("event sequence length mismatch: got %v\nwant %v", seen, want)
	}
	for i, ev := range want {
		if seen[i] != ev {
			t.Errorf("event[%d]: got %s, want %s", i, seen[i], ev)
		}
	}
}

func TestResponsesSSE_SequenceNumbersMonotonic(t *testing.T) {
	var buf bytes.Buffer
	r := NewResponsesSSEWriter(&buf, nil, "m")
	r.Feed(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "hi"}}}})
	r.Close()
	events := parseSSEEvents(t, buf.String())
	prev := 0
	for _, e := range events {
		sn, _ := e.Data["sequence_number"].(float64)
		if int(sn) <= prev {
			t.Fatalf("sequence_number not monotonic: prev=%d cur=%v", prev, sn)
		}
		prev = int(sn)
	}
}

func TestResponsesSSE_ReasoningEmitsThinkingItem(t *testing.T) {
	var buf bytes.Buffer
	r := NewResponsesSSEWriter(&buf, nil, "m")
	r.Feed(map[string]any{
		"choices": []any{
			map[string]any{"delta": map[string]any{"reasoning_content": "thinking…"}},
		},
	})
	r.Close()

	events := parseSSEEvents(t, buf.String())
	foundReasoningItem := false
	foundReasoningDelta := false
	for _, e := range events {
		if e.Event == "response.output_item.added" {
			item, _ := e.Data["item"].(map[string]any)
			if item["type"] == "reasoning" {
				foundReasoningItem = true
			}
		}
		if e.Event == "response.reasoning_summary_text.delta" {
			foundReasoningDelta = true
		}
	}
	if !foundReasoningItem {
		t.Error("missing reasoning output_item.added")
	}
	if !foundReasoningDelta {
		t.Error("missing reasoning_summary_text.delta")
	}
}

func TestResponsesSSE_ToolCallEmitsArgumentsDelta(t *testing.T) {
	var buf bytes.Buffer
	r := NewResponsesSSEWriter(&buf, nil, "m")
	r.Feed(map[string]any{
		"choices": []any{
			map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{
						map[string]any{
							"index":    0.0,
							"id":       "call_1",
							"function": map[string]any{"name": "get_w", "arguments": "{\"x\":1}"},
						},
					},
				},
			},
		},
	})
	r.Close()

	events := parseSSEEvents(t, buf.String())
	var foundFuncItem, foundArgsDelta, foundArgsDone bool
	for _, e := range events {
		if e.Event == "response.output_item.added" {
			if item, _ := e.Data["item"].(map[string]any); item["type"] == "function_call" {
				foundFuncItem = true
			}
		}
		if e.Event == "response.function_call_arguments.delta" {
			foundArgsDelta = true
		}
		if e.Event == "response.function_call_arguments.done" {
			foundArgsDone = true
		}
	}
	if !foundFuncItem || !foundArgsDelta || !foundArgsDone {
		t.Fatalf("tool-call events incomplete: item=%v delta=%v done=%v",
			foundFuncItem, foundArgsDelta, foundArgsDone)
	}
}

func TestResponsesSSE_UsagePreservedOnCompleted(t *testing.T) {
	var buf bytes.Buffer
	r := NewResponsesSSEWriter(&buf, nil, "m")
	r.Feed(map[string]any{
		"choices": []any{map[string]any{"delta": map[string]any{"content": "hi"}}},
		"usage":   map[string]any{"prompt_tokens": 10.0, "completion_tokens": 2.0, "total_tokens": 12.0},
	})
	r.Close()
	events := parseSSEEvents(t, buf.String())
	last := events[len(events)-1]
	resp := last.Data["response"].(map[string]any)
	usage, ok := resp["usage"].(map[string]any)
	if !ok {
		t.Fatal("usage missing from response.completed")
	}
	if usage["total_tokens"].(float64) != 12 {
		t.Errorf("total_tokens drift: %v", usage["total_tokens"])
	}
}

func TestResponsesSSE_CloseIsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	r := NewResponsesSSEWriter(&buf, nil, "m")
	r.Feed(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "hi"}}}})
	r.Close()
	sizeAfterFirst := buf.Len()
	r.Close()
	if buf.Len() != sizeAfterFirst {
		t.Fatalf("second Close() emitted more bytes: %d → %d", sizeAfterFirst, buf.Len())
	}
}
