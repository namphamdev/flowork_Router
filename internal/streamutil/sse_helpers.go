// SSE-line + chunk helpers shared across the streaming pipeline.
//
// Three exported helpers:
//
//	ParseSSELine(line, format)   → decode one wire-format line into a map
//	HasValuableContent(chunk, fmt) → tell whether a chunk carries real signal
//	FixInvalidID(chunk)          → patch generic/too-short ids in-place

package streamutil

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Stream formats this package understands. Callers pass these as the
// `format` argument so per-format quirks (NDJSON for Ollama, "data:" prefix
// for OpenAI/Claude/Gemini) get the right parsing path.
const (
	FormatOpenAI    = "openai"
	FormatClaude    = "claude"
	FormatGemini    = "gemini"
	FormatResponses = "openai-responses"
	FormatOllama    = "ollama"
)

// SSEParsed is the result of ParseSSELine. Either Object is set (the
// decoded JSON object) or Done is true (`data: [DONE]` sentinel).
type SSEParsed struct {
	Object map[string]any
	Done   bool
}

// ParseSSELine decodes one line of an upstream stream into a chunk object.
// Returns nil when the line carries no payload (empty / comment / wrong
// prefix / malformed JSON).
//
// Ollama format uses raw NDJSON (no `data:` prefix); every other format
// uses the standard `data: {…}` SSE shape with `[DONE]` as the sentinel.
func ParseSSELine(line, format string) *SSEParsed {
	if line == "" {
		return nil
	}

	// NDJSON — Ollama style.
	if format == FormatOllama {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "{") {
			return nil
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
			return nil
		}
		return &SSEParsed{Object: obj}
	}

	// Standard SSE — must start with 'd' (data:).
	if line[0] != 'd' || !strings.HasPrefix(line, "data:") {
		return nil
	}
	data := strings.TrimSpace(line[len("data:"):])
	if data == "" {
		return nil
	}
	if data == "[DONE]" {
		return &SSEParsed{Done: true}
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil
	}
	return &SSEParsed{Object: obj}
}

// HasValuableContent reports whether chunk carries real signal — non-empty
// content delta, reasoning delta, tool_calls activity, role announcement,
// or a finish_reason. Used to filter heartbeats / no-op deltas before
// piping to clients (saves wire bandwidth + avoids inflating chunk counts).
//
// Returns true for unknown formats so callers default to keep-everything
// rather than silently dropping signal.
func HasValuableContent(chunk map[string]any, format string) bool {
	if chunk == nil {
		return false
	}
	switch format {
	case FormatOpenAI:
		choices, _ := chunk["choices"].([]any)
		if len(choices) == 0 {
			return false
		}
		choice, _ := choices[0].(map[string]any)
		if fr, _ := choice["finish_reason"].(string); fr != "" {
			return true
		}
		delta, _ := choice["delta"].(map[string]any)
		if delta == nil {
			return false
		}
		if c, _ := delta["content"].(string); c != "" {
			return true
		}
		if r, _ := delta["reasoning_content"].(string); r != "" {
			return true
		}
		if tcs, _ := delta["tool_calls"].([]any); len(tcs) > 0 {
			return true
		}
		if role, _ := delta["role"].(string); role != "" {
			return true
		}
		return false

	case FormatClaude:
		typ, _ := chunk["type"].(string)
		if typ != "content_block_delta" {
			return true // non-delta events (message_start, etc.) are always kept
		}
		delta, _ := chunk["delta"].(map[string]any)
		if delta == nil {
			return false
		}
		if t, _ := delta["text"].(string); t != "" {
			return true
		}
		if t, _ := delta["thinking"].(string); t != "" {
			return true
		}
		if p, _ := delta["partial_json"].(string); p != "" {
			return true
		}
		return false

	default:
		return true
	}
}

// FixInvalidID rewrites chunk["id"] when it's the generic placeholder "chat"
// or "completion", or any value shorter than 8 characters. The replacement
// is `chatcmpl-<fallback>` where fallback is, in order of preference:
// extend_fields.requestId / extend_fields.traceId / a fresh base36 stamp.
// Returns true when the id was changed.
func FixInvalidID(chunk map[string]any) bool {
	if chunk == nil {
		return false
	}
	id, _ := chunk["id"].(string)
	if !isInvalidID(id) {
		return false
	}
	chunk["id"] = "chatcmpl-" + chooseFallbackID(chunk)
	return true
}

// isInvalidID reports whether id is one of the generic placeholders or too
// short to be a stable identifier.
func isInvalidID(id string) bool {
	if id == "" {
		return false // missing id is a different concern (caller may want to mint a new one upstream)
	}
	if id == "chat" || id == "completion" {
		return true
	}
	return len(id) < 8
}

// chooseFallbackID extracts a stable token from the chunk's extend_fields, or
// falls back to a base36 timestamp when neither requestId nor traceId is
// present. Never returns "".
func chooseFallbackID(chunk map[string]any) string {
	if ef, ok := chunk["extend_fields"].(map[string]any); ok {
		if v, _ := ef["requestId"].(string); v != "" {
			return v
		}
		if v, _ := ef["traceId"].(string); v != "" {
			return v
		}
	}
	// base36 of nanos so we don't share state with the streaming generator.
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
