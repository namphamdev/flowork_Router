// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 7 (HTTP boundary) phase 1 DONE + audit passed.
//   Endpoints stable: POST /api/mistakes/submit body {agent_id,
//   category, title, content, hit_count}, GET /api/mistakes?tier=
//   &source=&limit=. MaxBytesReader 32KB, DisallowUnknownFields.
//   Future /api/mistakes/promote → tambah file baru, JANGAN modify.
//
// handlers_brain_mistakes.go — Section 7 roadmap: Mistakes journal
// global endpoints. POST submit (receive promotion from agent) + GET list.
//
// Roadmap:
//   - internal/brain/mistakes.go (SubmitMistake / ListMistakes / CountMistakes)
//   - flowork_Router/roadmap.md Section 7

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/brain"
)

const maxMistakeBodyBytes = 32 * 1024

// brainMistakesSubmitHandler — POST /api/mistakes/submit
// Body: {agent_id, category, title, content, hit_count}.
// Return {id, added bool}. Validate hit_count ≥ 3 + category whitelist.
func brainMistakesSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxMistakeBodyBytes)

	var body struct {
		AgentID  string `json:"agent_id"`
		Category string `json:"category"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		HitCount int64  `json:"hit_count"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}
	id, isNew, err := brain.SubmitMistake(r.Context(), body.Category, body.Title,
		body.Content, body.AgentID, body.HitCount)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "added": isNew})
}

// brainMistakesListHandler — GET /api/mistakes?tier=&source=&limit=
// List mistakes journal — default 50, max 500.
//
// ⚠️ Anti over-prompt: list endpoint untuk dashboard/admin. JANGAN
// auto-inject ke chat — pakai semantic match query future.
func brainMistakesListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed (use GET)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	tier := strings.TrimSpace(r.URL.Query().Get("tier"))
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	limit := 50
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if n, perr := strconv.Atoi(s); perr == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	items, err := brain.ListMistakes(r.Context(), tier, source, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}
