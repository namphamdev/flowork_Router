// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 10 (HTTP boundary) phase 1 DONE. Endpoints stable:
//   POST /api/recordings (admin manual insert), GET /api/recordings
//   list with model/agent filter + include_body toggle, GET
//   /api/recordings/get?id= single with body. MaxBytesReader 128KB,
//   DisallowUnknownFields. Future router_rules/proxy/verifier
//   endpoints → tambah file baru, JANGAN modify ini.
//
// handlers_recordings.go — Section 10 phase 1: recordings endpoints.
//
// Endpoints:
//   POST /api/recordings — manual insert (admin)
//   GET  /api/recordings?model=&status=&limit=&include_body=1
//   GET  /api/recordings/get?id=<n> — single by ID with body
//
// Roadmap:
//   - internal/recorder/recorder.go (Save / List / Get / Count)
//   - flowork_Router/roadmap.md Section 10 phase 1

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/recorder"
)

const maxRecordingBodyBytes = 128 * 1024

// recordingsPostHandler — POST /api/recordings
// Body: RecordOpts (model, provider, request_body, response_text, ...).
func recordingsPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed (use POST)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRecordingBodyBytes)

	var body struct {
		Model        string          `json:"model"`
		RequestBody  json.RawMessage `json:"request_body"`
		ResponseText string          `json:"response_text"`
		InputTokens  int64           `json:"input_tokens"`
		OutputTokens int64           `json:"output_tokens"`
		CostUSD      float64         `json:"cost_usd"`
		BuildPass    int64           `json:"build_pass"`
		ToolCalls    []any           `json:"tool_calls"`
		Agent        string          `json:"agent"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
		return
	}

	// RequestBody RawMessage → marshal-friendly any.
	var reqBody any
	if len(body.RequestBody) > 0 {
		var tmp any
		if err := json.Unmarshal(body.RequestBody, &tmp); err == nil {
			reqBody = tmp
		}
	}

	id, err := recorder.Save(r.Context(), recorder.RecordOpts{
		Model:        body.Model,
		RequestBody:  reqBody,
		ResponseText: body.ResponseText,
		InputTokens:  body.InputTokens,
		OutputTokens: body.OutputTokens,
		CostUSD:      body.CostUSD,
		BuildPass:    body.BuildPass,
		ToolCalls:    body.ToolCalls,
		Agent:        body.Agent,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "algo_version": recorder.AlgoVersion})
}

// recordingsListHandler — GET /api/recordings?model=&status=&limit=&include_body=1
func recordingsListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed (use GET)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	opts := recorder.ListOpts{
		Model:       strings.TrimSpace(r.URL.Query().Get("model")),
		Agent:       strings.TrimSpace(r.URL.Query().Get("agent")),
		IncludeBody: r.URL.Query().Get("include_body") == "1",
	}
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if n, perr := strconv.Atoi(s); perr == nil && n > 0 && n <= 500 {
			opts.Limit = n
		}
	}
	items, err := recorder.List(r.Context(), opts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

// recordingsGetHandler — GET /api/recordings/get?id=<n>
func recordingsGetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed (use GET)", http.StatusMethodNotAllowed)
		return
	}
	if !ensureBrainReady(w, r) {
		return
	}
	idStr := strings.TrimSpace(r.URL.Query().Get("id"))
	id, perr := strconv.ParseInt(idStr, 10, 64)
	if perr != nil || id <= 0 {
		http.Error(w, "id required (positive int)", http.StatusBadRequest)
		return
	}
	rec, err := recorder.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if rec.Model == "" {
		http.Error(w, "recording not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}
