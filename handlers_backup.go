// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — HTTP handler. Method validation + JSON response + error handling per Router convention.

// Database Backup HTTP Handlers.

package main

import (
	"encoding/json"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/store"
)

func settingsBackupsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := store.ListBackups()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backups": list, "count": len(list)})
	case http.MethodPost:
		var body struct {
			Label string `json:"label"`
			KeepN int    `json:"keepN"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		info, err := store.Backup(body.Label, body.KeepN)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, info)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
