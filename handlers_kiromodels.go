// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — HTTP handler. Method validation + JSON response per Router convention.

// Kiro model discovery handler.

package main

import (
	"context"
	"net/http"
	"time"

	"github.com/flowork-os/flowork_Router/internal/kiromodels"
)

func kiroModelsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token := r.URL.Query().Get("token")
		if token == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "token query param required (Kiro OAuth access_token)",
			})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		cat, err := kiromodels.Fetch(ctx, kiromodels.Params{
			Token:      token,
			ProfileArn: r.URL.Query().Get("profileArn"),
			Region:     r.URL.Query().Get("region"),
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, cat)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func kiroModelsInvalidateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	kiromodels.InvalidateCache()
	writeJSON(w, http.StatusOK, map[string]any{"invalidated": true})
}
