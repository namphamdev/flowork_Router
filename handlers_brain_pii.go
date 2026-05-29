// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 3 (PII HTTP boundary) phase 1 DONE + audit passed.
//   Endpoint stable: POST /api/brain/pii/strip body {content, quiet?}.
//   MaxBytesReader 256KB, DisallowUnknownFields anti-typo, quiet branch
//   excludes Found samples (no raw-PII leak di response). Future
//   /api/brain/pii/audit endpoint → tambah file baru, JANGAN modify ini.
//
// handlers_brain_pii.go — Section 3 roadmap: PII strip admin endpoint.
//
// Roadmap:
//   - internal/piistrip/piistrip.go (regex library)
//   - flowork_Router/roadmap.md Section 3

package main

import (
	"encoding/json"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/piistrip"
)

// maxPIIBodyBytes — body cap untuk endpoint strip. 256KB sufficient
// untuk single drawer content.
const maxPIIBodyBytes = 256 * 1024

// brainPIIStripHandler — POST /api/brain/pii/strip
// Body: {"content": "...", "quiet": false}. Return piistrip.Result.
// `quiet=true` → omit Found samples (production mode, no raw PII di response).
//
// ⚠️ Debug-only endpoint — production ingestion pakai piistrip.StripQuiet
// di-call dari handler ingest (defer ke phase 2, ingest.Submit locked).
func brainPIIStripHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPIIBodyBytes)

	var body struct {
		Content string `json:"content"`
		Quiet   bool   `json:"quiet"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if body.Quiet {
		cleaned, counts, total := piistrip.StripQuiet(body.Content)
		writeJSON(w, http.StatusOK, map[string]any{
			"algo_version": piistrip.AlgoVersion,
			"cleaned":      cleaned,
			"counts":       counts,
			"total":        total,
		})
		return
	}
	result := piistrip.Strip(body.Content)
	writeJSON(w, http.StatusOK, result)
}
