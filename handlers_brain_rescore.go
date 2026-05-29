// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 2 (Re-score HTTP boundary) phase 1 DONE + audit
//   passed. Endpoint stable: POST /api/brain/rescore body
//   {wing, limit, force_override}. MaxBytesReader 16KB, chunked-body
//   tolerated, double cap (handler 5000 + brain internal 10000).
//   Future cron endpoint → tambah file baru, JANGAN modify.
//
// handlers_brain_rescore.go — Section 2 roadmap: re-score importance
// admin endpoint. Iterate live drawers, recompute via ingest.Score,
// UPDATE drawers.importance.
//
// Lihat:
//   - flowork_Router/roadmap.md Section 2 (Importance scorer)
//   - internal/brain/rescore.go (RescoreBatch + RescoreReport)
//   - internal/ingest/score.go (Score heuristic — LOCKED Section 1)

package main

import (
	"encoding/json"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/brain"
	"github.com/flowork-os/flowork_Router/internal/ingest"
)

// maxRescoreLimit — handler-level cap independen dari brain.RescoreBatch
// internal cap (defense in depth). 5000 cukup buat admin batch satu wing.
const maxRescoreLimit = 5000

// brainRescoreHandler — POST /api/brain/rescore
// Body: {"wing": "...", "limit": N} — wing filter optional, limit default 1000.
// Return brain.RescoreReport dengan stats + sample_delta first 20.
func brainRescoreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	// Anti memory blow: body kecil — paling cuma {wing, limit}.
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	var body struct {
		Wing          string `json:"wing"`
		Limit         int    `json:"limit"`
		ForceOverride bool   `json:"force_override"`
	}
	// Empty body OK — pakai defaults. ContentLength = -1 (chunked) tetap
	// di-decode best-effort; io.EOF di-tolerate jadi empty body fallback ke default.
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
			http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}
	if body.Limit < 0 || body.Limit > maxRescoreLimit {
		http.Error(w, "limit out of range (0 = default 1000, max 5000)", http.StatusBadRequest)
		return
	}

	report, err := brain.RescoreBatch(r.Context(), brain.RescoreOpts{
		Wing:          body.Wing,
		Limit:         body.Limit,
		ForceOverride: body.ForceOverride,
	}, ingest.Score)
	if err != nil {
		// Partial report bisa juga ada walau error — kembalikan dua-duanya.
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  err.Error(),
			"report": report,
		})
		return
	}
	writeJSON(w, http.StatusOK, report)
}
