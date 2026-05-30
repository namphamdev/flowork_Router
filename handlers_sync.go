// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — HTTP handler.

// Cross-Device Sync Endpoints.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flowork-os/flowork_Router/internal/store"
)

func syncExportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d, _ := store.Open()
	writeJSON(w, http.StatusOK, store.ExportConfig(d))
}

func syncImportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var b store.SyncBundle
	if err := json.NewDecoder(io.LimitReader(r.Body, 32*1024*1024)).Decode(&b); err != nil {
		http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
		return
	}
	d, _ := store.Open()
	writeJSON(w, http.StatusOK, map[string]any{"imported": store.ImportConfig(d, &b)})
}

// syncPullHandler — fetch {from}/api/sync/export and import it.
func syncPullHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		From  string `json:"from"`
		Token string `json:"token,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.From == "" {
		http.Error(w, "from required (e.g. http://other-host:2402)", http.StatusBadRequest)
		return
	}
	endpoint := strings.TrimRight(body.From, "/") + "/api/sync/export"
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		http.Error(w, "request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Token != "" {
		req.Header.Set("Authorization", "Bearer "+body.Token)
	}
	resp, err := providerProbeClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "pull: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if resp.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "remote export " + http.StatusText(resp.StatusCode), "status": resp.StatusCode})
		return
	}
	var b store.SyncBundle
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&b); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "remote bundle parse: " + err.Error()})
		return
	}
	d, _ := store.Open()
	writeJSON(w, http.StatusOK, map[string]any{"from": body.From, "imported": store.ImportConfig(d, &b)})
}
