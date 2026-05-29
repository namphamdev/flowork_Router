// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 4 (HTTP boundary) phase 1 DONE + audit passed.
//   Endpoint stable: POST /api/brain/injection/check body {content}.
//   MaxBytesReader 128KB, DisallowUnknownFields. Future quarantine
//   workflow endpoint → tambah file baru, JANGAN modify ini.
//
// handlers_brain_injection.go — Section 4 roadmap: Prompt injection
// detector admin endpoint.
//
// Roadmap:
//   - internal/promptguard/promptguard.go (signature library)
//   - flowork_Router/roadmap.md Section 4

package main

import (
	"encoding/json"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/promptguard"
)

const maxInjectionBodyBytes = 128 * 1024

// brainInjectionCheckHandler — POST /api/brain/injection/check
// Body: {"content": "..."}. Return promptguard.Result (severity, score, hits).
func brainInjectionCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxInjectionBodyBytes)

	var body struct {
		Content string `json:"content"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}
	result := promptguard.Detect(body.Content)
	writeJSON(w, http.StatusOK, result)
}
