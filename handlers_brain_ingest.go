// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 1 (Ingestion HTTP boundary) DONE + adversarial-audit passed.
//   MaxBytesReader 16MB anti memory blow, ensureBrainReady guard. Endpoint
//   stable: /submit + /batch. Section 4 worker brain compounding (extend
//   /api/brain/ingest/run di handlers_brain_views.go — file beda). Future
//   /api/brain/ingest/federation → tambah handler baru di file lain, JANGAN
//   modify file ini tanpa approval.
//
// handlers_brain_ingest.go — endpoint POST /api/brain/ingest/submit
// dan /api/brain/ingest/batch untuk grow brain via external/API caller.
//
// Lihat:
//   - flowork_Router/roadmap.md Section 1 (Ingestion pipeline)
//   - internal/ingest/ (pipeline orchestrator)
//   - internal/brain/write.go::AddDrawerFull (write primitive)
//
// Existing /api/brain/ingest/run (handlers_brain_views.go) untuk compounding
// dari interaction contributions — tetap independen.

package main

import (
	"encoding/json"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/brain"
	"github.com/flowork-os/flowork_Router/internal/ingest"
	"github.com/flowork-os/flowork_Router/internal/store"
)

// maxBatchItems — hard cap berapa drawer per /batch call. Cegah satu request
// monopoli single writer SQLite. Caller chunk ulang kalau perlu.
const maxBatchItems = 1000

// maxIngestBodyBytes — hard cap body POST. Cegah caller spam giant JSON
// → OOM. 16MB cukup untuk batch 1000 item content rata-rata 16KB.
const maxIngestBodyBytes = 16 << 20

// brainIngestSubmitHandler — POST /api/brain/ingest/submit
// Body: ingest.Req (content + opsional wing/room/source_type/source_file/
// mem_type/importance/chunk_index). Return drawer_id + added flag.
func brainIngestSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxIngestBodyBytes)

	var req ingest.Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}
	res := ingest.Submit(r.Context(), req)
	if res.Error != "" {
		writeJSON(w, http.StatusBadRequest, res)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// brainIngestBatchHandler — POST /api/brain/ingest/batch
// Body: {"items": [ingest.Req, ...]}. Return per-item Result + agregat stats.
//
// Tidak short-circuit pada error individual — caller bisa pilih item mana
// yang retry. Cap di maxBatchItems supaya satu request ngga monopoli writer.
func brainIngestBatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxIngestBodyBytes)

	var body struct {
		Items []ingest.Req `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(body.Items) == 0 {
		http.Error(w, "items required (non-empty)", http.StatusBadRequest)
		return
	}
	if len(body.Items) > maxBatchItems {
		http.Error(w, "items > max batch size", http.StatusRequestEntityTooLarge)
		return
	}

	results := ingest.SubmitBatch(r.Context(), body.Items)
	writeJSON(w, http.StatusOK, map[string]any{
		"stats":   ingest.Summarize(results),
		"results": results,
	})
}

// ensureBrainReady — guard helper dipakai semua ingest handler. Apply path
// dari settings + cek brain.Available(). Return true kalau ready, false kalau
// response error udah di-write.
func ensureBrainReady(w http.ResponseWriter, _ *http.Request) bool {
	d, _ := store.Open()
	s, _ := store.LoadSettings(d)
	applyBrainPath(s)
	if !brain.Available() {
		http.Error(w, "brain DB not available", http.StatusServiceUnavailable)
		return false
	}
	return true
}
