// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 6 (HTTP boundary) phase 1 DONE. Endpoints stable:
//   POST /api/brain/tool-patterns/learn body {trigger, tool_name,
//   success} return {ok, amplitude}. GET /api/brain/tool-patterns
//   ?trigger=&limit= return ranked list (max 10). MaxBytesReader 16KB,
//   DisallowUnknownFields. Future endpoints → tambah file baru.
//
// handlers_brain_tools.go — Section 6 roadmap: Tool learner endpoints.
//
// Roadmap:
//   - internal/brain/tool_patterns.go (LearnPattern + SuggestTools)
//   - flowork_Router/roadmap.md Section 6

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/brain"
)

const maxToolLearnBodyBytes = 16 * 1024

// brainToolLearnHandler — POST /api/brain/tool-patterns/learn
// Body: {trigger, tool_name, success bool}. Upsert pattern, return new
// amplitude.
func brainToolLearnHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxToolLearnBodyBytes)

	var body struct {
		Trigger  string `json:"trigger"`
		ToolName string `json:"tool_name"`
		Success  bool   `json:"success"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
		return
	}
	amp, err := brain.LearnPattern(r.Context(), body.Trigger, body.ToolName, body.Success)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"amplitude": amp,
	})
}

// brainToolSuggestHandler — GET /api/brain/tool-patterns?trigger=<text>&limit=
// Return ranked ToolPattern list (max 10).
func brainToolSuggestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed (use GET)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	trigger := strings.TrimSpace(r.URL.Query().Get("trigger"))
	if trigger == "" {
		http.Error(w, "trigger required", http.StatusBadRequest)
		return
	}
	limit := 5
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if n, perr := strconv.Atoi(s); perr == nil && n > 0 && n <= 10 {
			limit = n
		}
	}
	items, err := brain.SuggestTools(r.Context(), trigger, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}
