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

func TestParseResponsesSSEToJSON_EmptyReturnsCompleted(t *testing.T) {
	out := ParseResponsesSSEToJSON(nil)
	if out["object"] != "response" {
		t.Fatalf("object: %v", out["object"])
	}
	if out["status"] != "completed" {
		t.Fatalf("empty stream should still resolve to completed, got %v", out["status"])
	}
}

func TestParseResponsesSSEToJSON_CapturesCreatedThenItem(t *testing.T) {
	body := `event: response.created
data: {"type":"response.created","response":{"id":"resp_42","created_at":1234567890}}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_42","status":"completed","usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}

`
	out := ParseResponsesSSEToJSON([]byte(body))
	if out["id"] != "resp_42" {
		t.Errorf("id mismatch: %v", out["id"])
	}
	if out["status"] != "completed" {
		t.Errorf("status: %v", out["status"])
	}
	output := out["output"].([]map[string]any)
	if len(output) != 1 {
		t.Fatalf("expected 1 item, got %d", len(output))
	}
	if output[0]["type"] != "message" {
		t.Errorf("item type: %v", output[0]["type"])
	}
	usage, _ := out["usage"].(map[string]any)
	if usage["total_tokens"].(float64) != 12 {
		t.Errorf("total_tokens lost: %v", usage["total_tokens"])
	}
}

func TestParseResponsesSSEToJSON_OrdersItemsByIndex(t *testing.T) {
	body := `event: response.output_item.done
data: {"type":"response.output_item.done","output_index":2,"item":{"id":"c","type":"message","role":"assistant"}}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"id":"a","type":"message","role":"assistant"}}

`
	out := ParseResponsesSSEToJSON([]byte(body))
	items := out["output"].([]map[string]any)
	if len(items) != 3 {
		t.Fatalf("expected 3 slots (gap-filled), got %d", len(items))
	}
	if items[0]["id"] != "a" {
		t.Errorf("slot[0]: %v", items[0]["id"])
	}
	if items[2]["id"] != "c" {
		t.Errorf("slot[2]: %v", items[2]["id"])
	}
	// Gap slot at index 1 must be a placeholder.
	if items[1]["type"] != "message" {
		t.Errorf("gap slot type: %v", items[1]["type"])
	}
}

func TestParseResponsesSSEToJSON_FailedStatus(t *testing.T) {
	body := `event: response.created
data: {"type":"response.created","response":{"id":"resp_x"}}

event: response.failed
data: {"type":"response.failed","response":{"error":"boom"}}

`
	out := ParseResponsesSSEToJSON([]byte(body))
	if out["status"] != "failed" {
		t.Fatalf("status: %v", out["status"])
	}
}

func TestParseResponsesSSEToJSON_IgnoresUnrelatedEvents(t *testing.T) {
	body := `event: response.created
data: {"type":"response.created","response":{"id":"resp_y","created_at":1000}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"hi"}

event: response.completed
data: {"type":"response.completed","response":{"status":"completed"}}

`
	out := ParseResponsesSSEToJSON([]byte(body))
	if out["id"] != "resp_y" {
		t.Errorf("id: %v", out["id"])
	}
	if out["status"] != "completed" {
		t.Errorf("status: %v", out["status"])
	}
	// We didn't see any output_item.done so output is empty.
	if items := out["output"].([]map[string]any); len(items) != 0 {
		t.Errorf("output should be empty without item.done events, got %d", len(items))
	}
}

func TestParseResponsesSSEToJSON_HandlesDoneSentinel(t *testing.T) {
	// Some upstreams send "data: [DONE]" — must be skipped, not parsed.
	body := `event: response.created
data: {"type":"response.created","response":{"id":"resp_a"}}

data: [DONE]
`
	out := ParseResponsesSSEToJSON([]byte(body))
	if out["id"] != "resp_a" {
		t.Fatalf("[DONE] sentinel must not abort parsing")
	}
}

func TestParseResponsesSSEToJSON_SynthesisesResponseIDWhenMissing(t *testing.T) {
	body := `event: response.completed
data: {"type":"response.completed","response":{"status":"completed"}}

`
	out := ParseResponsesSSEToJSON([]byte(body))
	id, _ := out["id"].(string)
	if !strings.HasPrefix(id, "resp_") {
		t.Fatalf("synthesised id should start with resp_, got %q", id)
	}
}
