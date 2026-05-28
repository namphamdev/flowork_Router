// OpenAI chat-completion SSE → Ollama JSON-lines stream translator.
// Native Ollama clients (the `ollama` CLI, libraries like ollama-python)
// connect to /api/chat expecting ndjson rows, not OpenAI SSE. This file
// owns the streaming conversion + the final "done" sentinel.

package response

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// ollamaToolAcc accumulates streaming tool_call deltas keyed by index.
type ollamaToolAcc struct {
	ID   string
	Name string
	Args string
}

// TransformOpenAISSEToOllamaNDJSON reads an OpenAI SSE stream from src and
// writes Ollama-format NDJSON rows to dst. Each row is one JSON object
// followed by a newline:
//
//	{"model":"…","message":{"role":"assistant","content":"…"},"done":false}
//
// and the final row carries done=true (plus any accumulated tool_calls).
// Returns the number of OpenAI chunks consumed (for logging/usage).
func TransformOpenAISSEToOllamaNDJSON(src io.Reader, dst io.Writer, model string) int {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)

	chunks := 0
	tools := map[int]*ollamaToolAcc{}
	finishReason := ""
	sentDone := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(line[len("data:"):])
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			emitOllamaDone(dst, model, tools, finishReason)
			sentDone = true
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // skip malformed
		}
		chunks++
		choices, _ := chunk["choices"].([]any)
		if len(choices) == 0 {
			continue
		}
		choice, _ := choices[0].(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		if fr, _ := choice["finish_reason"].(string); fr != "" {
			finishReason = fr
		}
		if tcs, ok := delta["tool_calls"].([]any); ok {
			for _, t := range tcs {
				tc, _ := t.(map[string]any)
				idxF, _ := tc["index"].(float64)
				idx := int(idxF)
				acc, has := tools[idx]
				if !has {
					acc = &ollamaToolAcc{}
					tools[idx] = acc
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
		// Per-chunk text delta → one Ollama row with done=false.
		if c, _ := delta["content"].(string); c != "" {
			row := map[string]any{
				"model": model,
				"message": map[string]any{
					"role":    "assistant",
					"content": c,
				},
				"done": false,
			}
			writeOllamaRow(dst, row)
		}
	}

	// Ensure the consumer sees exactly one done=true row even when [DONE]
	// was absent (upstream cut without the sentinel).
	if !sentDone {
		emitOllamaDone(dst, model, tools, finishReason)
	}
	return chunks
}

// emitOllamaDone writes the final row carrying any accumulated tool_calls.
// Tool args are parsed from the streamed JSON string back into a value so
// Ollama clients see them as a nested object (matching the native shape).
func emitOllamaDone(w io.Writer, model string, tools map[int]*ollamaToolAcc, finishReason string) {
	message := map[string]any{"role": "assistant", "content": ""}
	if len(tools) > 0 {
		calls := make([]map[string]any, 0, len(tools))
		// Order by index for determinism.
		maxIdx := -1
		for i := range tools {
			if i > maxIdx {
				maxIdx = i
			}
		}
		for i := 0; i <= maxIdx; i++ {
			acc, ok := tools[i]
			if !ok {
				continue
			}
			var args any
			if acc.Args != "" {
				if err := json.Unmarshal([]byte(acc.Args), &args); err != nil {
					args = map[string]any{}
				}
			} else {
				args = map[string]any{}
			}
			calls = append(calls, map[string]any{
				"function": map[string]any{
					"name":      acc.Name,
					"arguments": args,
				},
			})
		}
		if len(calls) > 0 {
			message["tool_calls"] = calls
		}
	}
	_ = finishReason // reserved for future "stop reason" mapping if Ollama adds one
	writeOllamaRow(w, map[string]any{
		"model":   model,
		"message": message,
		"done":    true,
	})
}

func writeOllamaRow(w io.Writer, row map[string]any) {
	raw, err := json.Marshal(row)
	if err != nil {
		return
	}
	_, _ = w.Write(raw)
	_, _ = w.Write([]byte{'\n'})
}

// TransformOpenAISSEToOllamaBytes is a buffer-based convenience around
// TransformOpenAISSEToOllamaNDJSON for callers that have the SSE body
// already materialised (non-streaming responses, tests, etc.).
func TransformOpenAISSEToOllamaBytes(sseBody []byte, model string) []byte {
	src := bytes.NewReader(sseBody)
	var dst bytes.Buffer
	TransformOpenAISSEToOllamaNDJSON(src, &dst, model)
	return dst.Bytes()
}
