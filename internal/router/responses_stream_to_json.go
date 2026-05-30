// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Router dispatcher.

// Collapse a Responses-API SSE stream into a single Responses JSON.
// Mirror of ParseSSEToOpenAIResponse but for the /v1/responses event shape
// (response.created / response.output_item.done / response.completed / …).

package router

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ParseResponsesSSEToJSON walks an event-stream emitted by the Responses API
// and returns the final single-JSON envelope. Used when the client asked
// for /v1/responses non-streaming but the upstream provider only speaks
// streaming (Codex backend forces stream=true).
//
// Event handling:
//
//	response.created          → captures id + created_at
//	response.output_item.done → stashes the completed item by output_index
//	response.completed        → marks status=completed + lifts usage
//	response.failed           → marks status=failed
//
// Other events (item.added, content_part.added, *_delta, *_done) are ignored
// — we only need the final item snapshots, which arrive inside
// response.output_item.done.
func ParseResponsesSSEToJSON(rawSSE []byte) map[string]any {
	state := struct {
		responseID string
		created    int64
		status     string
		usage      map[string]any
		items      map[int]map[string]any
	}{
		created: time.Now().Unix(),
		status:  "in_progress",
		usage:   map[string]any{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0},
		items:   map[int]map[string]any{},
	}

	// Split on blank-line delimiters which separate SSE events.
	for _, block := range strings.Split(string(rawSSE), "\n\n") {
		processResponsesSSEBlock(block, &state.responseID, &state.created, &state.status, &state.usage, state.items)
	}

	// Build output array ordered by output_index. Gaps are filled with empty
	// assistant message slots so downstream consumers don't trip on holes.
	maxIdx := -1
	for i := range state.items {
		if i > maxIdx {
			maxIdx = i
		}
	}
	output := make([]map[string]any, 0, maxIdx+1)
	for i := 0; i <= maxIdx; i++ {
		if it, ok := state.items[i]; ok {
			output = append(output, it)
		} else {
			output = append(output, map[string]any{
				"type":    "message",
				"role":    "assistant",
				"content": []map[string]any{},
			})
		}
	}
	if state.responseID == "" {
		state.responseID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
	}
	if state.status == "in_progress" {
		// No response.completed observed — assume completed but with no usage.
		state.status = "completed"
	}
	return map[string]any{
		"id":         state.responseID,
		"object":     "response",
		"created_at": state.created,
		"status":     state.status,
		"output":     output,
		"usage":      state.usage,
	}
}

// processResponsesSSEBlock parses a single SSE event block and mutates the
// caller-supplied state slots.
func processResponsesSSEBlock(block string, respID *string, created *int64, status *string, usage *map[string]any, items map[int]map[string]any) {
	var eventType, dataStr string
	for _, line := range strings.Split(strings.TrimSpace(block), "\n") {
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(line[len("event:"):])
		} else if strings.HasPrefix(line, "data:") {
			dataStr = strings.TrimSpace(line[len("data:"):])
		}
	}
	if dataStr == "" || dataStr == "[DONE]" || eventType == "" {
		return
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return
	}

	switch eventType {
	case "response.created":
		if r, ok := data["response"].(map[string]any); ok {
			if id, _ := r["id"].(string); id != "" {
				*respID = id
			}
			if c, ok := r["created_at"].(float64); ok && c > 0 {
				*created = int64(c)
			}
		}

	case "response.output_item.done":
		idxF, _ := data["output_index"].(float64)
		if it, ok := data["item"].(map[string]any); ok {
			items[int(idxF)] = it
		}

	case "response.completed":
		*status = "completed"
		if r, ok := data["response"].(map[string]any); ok {
			if u, ok := r["usage"].(map[string]any); ok {
				*usage = u
			}
		}

	case "response.failed":
		*status = "failed"
	}
}
