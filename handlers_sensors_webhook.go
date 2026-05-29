// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 9 (HTTP webhook boundary) phase 1 DONE. Endpoint
//   stable: POST /api/sensors/webhook?source= header X-Sensor-Token,
//   body raw content → ingest.Submit dengan wing='webhook' room=source.
//   MaxBytesReader 256KB. Constant-time token compare anti timing
//   attack. Future file watcher / scheduler endpoints → tambah file
//   baru, JANGAN modify ini.
//
// handlers_sensors_webhook.go — Section 9 roadmap: webhook receiver.
//
// POST /api/sensors/webhook?source=<id>
//   Header: X-Sensor-Token: <token>
//   Body:   content text (UTF-8)
//
// Validate token → forward content ke ingest.Submit (source_type='webhook').
//
// Roadmap:
//   - internal/sensors/sensors.go (AuthSource + token validation)
//   - internal/ingest/ingest.go (Submit pipeline — LOCKED)
//   - flowork_Router/roadmap.md Section 9

package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/ingest"
	"github.com/flowork-os/flowork_Router/internal/sensors"
)

const maxWebhookBodyBytes = 256 * 1024

// sensorsWebhookHandler — POST /api/sensors/webhook?source=<id>
func sensorsWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)

	sourceID := strings.TrimSpace(r.URL.Query().Get("source"))
	if sourceID == "" {
		http.Error(w, "source required", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(r.Header.Get("X-Sensor-Token"))
	if token == "" {
		http.Error(w, "X-Sensor-Token header required", http.StatusUnauthorized)
		return
	}
	if err := sensors.AuthSource(sourceID, token); err != nil {
		// Opaque error untuk client. Internal log via log.Printf (caller).
		status := http.StatusUnauthorized
		if errors.Is(err, sensors.ErrInvalidSourceID) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	// Body content — read raw bytes.
	bodyBytes, rerr := io.ReadAll(r.Body)
	if rerr != nil {
		http.Error(w, "read body: "+rerr.Error(), http.StatusBadRequest)
		return
	}
	content := string(bodyBytes)
	if strings.TrimSpace(content) == "" {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// Submit via existing ingest pipeline. SourceType='webhook', wing
	// hardcoded 'webhook', room = source ID supaya bisa di-filter / browse.
	res := ingest.Submit(r.Context(), ingest.Req{
		Content:    content,
		Wing:       "webhook",
		Room:       sourceID,
		SourceType: "webhook",
		SourceFile: sourceID,
	})
	if res.Error != "" {
		writeJSON(w, http.StatusBadRequest, res)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"algo_version": sensors.AlgoVersion,
		"drawer_id":    res.DrawerID,
		"added":        res.Added,
		"note":         res.Note,
	})
}

// Compile-time check: json package imported (kept for future webhook variants
// dengan structured payload).
var _ = json.Marshal
