// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 5 (Quality HTTP boundary) phase 1 DONE + audit passed.
//   POST /api/brain/quality/check. MaxBytesReader 512KB,
//   DisallowUnknownFields anti-typo, pure passthrough ke quality.Check.
//   Future endpoint (batch check, embedding integration) → tambah file
//   baru, JANGAN modify ini.
//
// handlers_brain_quality.go — Section 5 roadmap: Quality gate admin
// endpoint. Caller invoke standalone untuk pre-ingest check.
//
// Roadmap:
//   - internal/quality/quality.go (heuristic library)
//   - flowork_Router/roadmap.md Section 5

package main

import (
	"encoding/json"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/quality"
)

// maxQualityBodyBytes — body cap untuk endpoint check. 512KB > maxLengthBytes
// (256KB) supaya quality.Check return reason "too long" instead of
// MaxBytesReader truncation.
const maxQualityBodyBytes = 512 * 1024

// brainQualityCheckHandler — POST /api/brain/quality/check
// Body: {"content": "..."}. Return quality.Result (Allowed + Reason + 4 sub-scores).
//
// No DB access — pure heuristic. Cepat (< 1ms typical).
func brainQualityCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxQualityBodyBytes)

	var body struct {
		Content string `json:"content"`
	}
	dec := json.NewDecoder(r.Body)
	// Audit fix: reject typo "contentt" / wrong field → caller dapat
	// debug feedback instead of silent empty-content reject.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}
	result := quality.Check(body.Content)
	writeJSON(w, http.StatusOK, result)
}
