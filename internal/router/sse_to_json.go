// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Router dispatcher.

// Aggregate an OpenAI-shape SSE stream into a single chat.completion JSON.
// Used when a provider is streaming-only but the client requested a non-
// streaming response — the router collects every `data: …` chunk, merges
// content + tool_calls + reasoning + usage, and returns one final body.

package router

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// sseToolCallAcc accumulates the (possibly streaming) tool_call deltas keyed
// by the index field that OpenAI uses to pair chunk-by-chunk arguments back
// to one logical call.
type sseToolCallAcc struct {
	ID   string
	Name string
	Args string
}

// ParseSSEToOpenAIResponse walks a raw SSE buffer (the whole stream body)
// and returns the aggregated OpenAI chat.completion object — or nil when the
// buffer carried no valid `data:` chunks. fallbackModel is used when the
// first chunk didn't include a model id.
//
// Recognised chunk shape per line:
//
//	data: {"id":"…","choices":[{"delta":{"content":"…"},"finish_reason":"…"}], "usage":{…}}
//
// Content is concatenated; tool_calls accumulate by index; reasoning_content
// is preserved on a separate field for transports that surface it.
func ParseSSEToOpenAIResponse(rawSSE []byte, fallbackModel string) map[string]any {
	if len(rawSSE) == 0 {
		return nil
	}
	var chunks []map[string]any
	for _, line := range strings.Split(string(rawSSE), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		payload := strings.TrimSpace(trimmed[len("data:"):])
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var ch map[string]any
		if err := json.Unmarshal([]byte(payload), &ch); err != nil {
			continue // skip malformed lines
		}
		chunks = append(chunks, ch)
	}
	if len(chunks) == 0 {
		return nil
	}

	first := chunks[0]
	var (
		content      strings.Builder
		reasoning    strings.Builder
		toolCalls    = map[int]*sseToolCallAcc{}
		finishReason = "stop"
		usage        map[string]any
	)

	for _, chunk := range chunks {
		choices, _ := chunk["choices"].([]any)
		if len(choices) == 0 {
			if u, ok := chunk["usage"].(map[string]any); ok {
				usage = u
			}
			continue
		}
		choice, _ := choices[0].(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		if c, _ := delta["content"].(string); c != "" {
			content.WriteString(c)
		}
		if r, _ := delta["reasoning_content"].(string); r != "" {
			reasoning.WriteString(r)
		}
		if fr, _ := choice["finish_reason"].(string); fr != "" {
			finishReason = fr
		}
		if u, ok := chunk["usage"].(map[string]any); ok {
			usage = u
		}
		if tcs, ok := delta["tool_calls"].([]any); ok {
			for _, t := range tcs {
				tc, _ := t.(map[string]any)
				idxF, _ := tc["index"].(float64)
				idx := int(idxF)
				acc, has := toolCalls[idx]
				if !has {
					acc = &sseToolCallAcc{}
					toolCalls[idx] = acc
				}
				if id, _ := tc["id"].(string); id != "" {
					acc.ID = id
				}
				if fn, ok := tc["function"].(map[string]any); ok {
					if n, _ := fn["name"].(string); n != "" {
						acc.Name += n
					}
					if a, _ := fn["arguments"].(string); a != "" {
						acc.Args += a
					}
				}
			}
		}
	}

	message := map[string]any{"role": "assistant"}
	if content.Len() > 0 {
		message["content"] = content.String()
	} else if len(toolCalls) > 0 {
		message["content"] = nil // OpenAI shape: null content when tool_calls only
	} else {
		message["content"] = ""
	}
	if reasoning.Len() > 0 {
		message["reasoning_content"] = reasoning.String()
	}
	if len(toolCalls) > 0 {
		idxs := make([]int, 0, len(toolCalls))
		for i := range toolCalls {
			idxs = append(idxs, i)
		}
		sort.Ints(idxs)
		out := make([]map[string]any, 0, len(idxs))
		for _, i := range idxs {
			acc := toolCalls[i]
			out = append(out, map[string]any{
				"id":   acc.ID,
				"type": "function",
				"function": map[string]any{
					"name":      acc.Name,
					"arguments": acc.Args,
				},
			})
		}
		message["tool_calls"] = out
	}

	id, _ := first["id"].(string)
	if id == "" {
		id = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}
	model, _ := first["model"].(string)
	if model == "" {
		model = fallbackModel
	}
	createdF, _ := first["created"].(float64)
	created := int64(createdF)
	if created == 0 {
		created = time.Now().Unix()
	}

	result := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": created,
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       message,
			"finish_reason": finishReason,
		}},
	}
	if usage != nil {
		result["usage"] = usage
	}
	return result
}
