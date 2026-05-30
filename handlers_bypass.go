// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — HTTP handler.

// Claude-CLI bypass HTTP adapter.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/flowork-os/flowork_Router/internal/bypass"
	"github.com/flowork-os/flowork_Router/internal/router"
	"github.com/flowork-os/flowork_Router/internal/store"
)

// tryClaudeCliBypass returns true when the request matched a no-op pattern
// AND a stub response was written to w. Caller MUST return immediately on
// true. Returns false when settings are off, UA doesn't match, or no pattern
// fires — in which case w has not been touched.
func tryClaudeCliBypass(w http.ResponseWriter, r *http.Request, req *router.OpenAIRequest) bool {
	d, err := store.Open()
	if err != nil {
		return false
	}
	settings, _ := store.LoadSettings(d)
	if settings == nil || !settings.ClaudeCliBypass.Enabled {
		return false
	}

	msgs := make([]bypass.Message, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = bypass.Message{Role: m.Role, Content: m.Content}
	}
	decision := bypass.Detect(
		msgs,
		r.Header.Get("User-Agent"),
		settings.ClaudeCliBypass.SkipPatterns,
		settings.ClaudeCliBypass.CcFilterNaming,
	)
	if !decision.Bypass {
		return false
	}

	stub := bypass.StubText(decision)
	if req.Stream {
		writeBypassSSE(w, req.Model, stub)
	} else {
		writeBypassJSON(w, req.Model, stub)
	}
	return true
}

// writeBypassJSON emits a single OpenAI chat.completion response with the
// stub as the assistant message.
func writeBypassJSON(w http.ResponseWriter, model, text string) {
	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	resp := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": text},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     1,
			"completion_tokens": 1,
			"total_tokens":      2,
		},
	}
	raw, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

// writeBypassSSE emits one delta chunk carrying the stub text plus the
// standard OpenAI stop chunk and [DONE] sentinel.
func writeBypassSSE(w http.ResponseWriter, model, text string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Caller doesn't support flushing — degrade to JSON.
		writeBypassJSON(w, model, text)
		return
	}
	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	delta := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"delta": map[string]any{"role": "assistant", "content": text},
		}},
	}
	stop := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": "stop",
		}},
	}
	raw, _ := json.Marshal(delta)
	_, _ = io.WriteString(w, "data: ")
	_, _ = w.Write(raw)
	_, _ = io.WriteString(w, "\n\n")
	raw, _ = json.Marshal(stop)
	_, _ = io.WriteString(w, "data: ")
	_, _ = w.Write(raw)
	_, _ = io.WriteString(w, "\n\ndata: [DONE]\n\n")
	flusher.Flush()
}
