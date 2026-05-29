// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Section 11 (HTTP boundary) phase 1 DONE. Namespace
//   /api/brain/models (anti-collision dengan existing /api/models
//   handlers_models_meta.go yang serve flowork-settings store).
//   Endpoints stable: POST upsert, GET list filter (category/is_free/
//   max_cost/limit), GET /get?id= single, DELETE. MaxBytesReader 16KB,
//   DisallowUnknownFields. Future refresh/resolve endpoints → tambah
//   file baru, JANGAN modify ini.
//
// handlers_brain_models.go — Section 11 phase 1: model pool CRUD.
//
// NAMESPACE /api/brain/models (NOT /api/models — avoid collision dengan
// existing handlers_models_meta.go yang serve flowork-settings store).
//
// Endpoints:
//   POST   /api/brain/models — upsert single (body: UpsertOpts)
//   GET    /api/brain/models?category=&is_free=1&max_cost=&limit=
//   GET    /api/brain/models/get?id=<model_id>
//   DELETE /api/brain/models?id=<model_id> — DESTRUCTIVE physical row remove
//
// Roadmap:
//   - internal/modelpool/modelpool.go (Upsert/Get/List/Delete/Count)
//   - flowork_Router/roadmap.md Section 11 phase 1

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/modelpool"
)

const maxBrainModelBodyBytes = 16 * 1024

// brainModelsHandler — dispatch /api/brain/models by method.
//   POST   → upsert
//   GET    → list
//   DELETE → remove
func brainModelsHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureBrainReady(w, r) {
		return
	}
	switch r.Method {
	case http.MethodPost:
		brainModelsPost(w, r)
	case http.MethodGet:
		brainModelsList(w, r)
	case http.MethodDelete:
		brainModelsDelete(w, r)
	default:
		http.Error(w, "method not allowed (POST/GET/DELETE)", http.StatusMethodNotAllowed)
	}
}

// brainModelsPost — POST /api/brain/models — upsert.
func brainModelsPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBrainModelBodyBytes)
	var body modelpool.UpsertOpts
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
		return
	}
	id, isNew, err := modelpool.Upsert(r.Context(), body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           id,
		"added":        isNew,
		"algo_version": modelpool.AlgoVersion,
	})
}

// brainModelsList — GET /api/brain/models?category=&is_free=1&max_cost=&limit=
func brainModelsList(w http.ResponseWriter, r *http.Request) {
	opts := modelpool.ListOpts{
		Category:   strings.TrimSpace(r.URL.Query().Get("category")),
		IsFreeOnly: r.URL.Query().Get("is_free") == "1",
	}
	if s := strings.TrimSpace(r.URL.Query().Get("max_cost")); s != "" {
		if f, perr := strconv.ParseFloat(s, 64); perr == nil && f > 0 {
			opts.MaxCost = f
		}
	}
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if n, perr := strconv.Atoi(s); perr == nil && n > 0 && n <= 500 {
			opts.Limit = n
		}
	}
	items, err := modelpool.List(r.Context(), opts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

// brainModelsDelete — DELETE /api/brain/models?id=<model_id> — DESTRUCTIVE.
func brainModelsDelete(w http.ResponseWriter, r *http.Request) {
	modelID := strings.TrimSpace(r.URL.Query().Get("id"))
	if modelID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	n, err := modelpool.Delete(r.Context(), modelID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if n == 0 {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": n, "model_id": modelID})
}

// brainModelsGetHandler — GET /api/brain/models/get?id=<model_id>
func brainModelsGetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed (use GET)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	modelID := strings.TrimSpace(r.URL.Query().Get("id"))
	if modelID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	m, err := modelpool.Get(r.Context(), modelID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if m.ModelName == "" {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
