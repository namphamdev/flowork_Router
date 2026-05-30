// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — HTTP handler.

// Brain dashboard API for the shared knowledge brain..

package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/flowork-os/flowork_Router/internal/brain"
	"github.com/flowork-os/flowork_Router/internal/store"
)

// applyBrainPath points the brain package at the configured DB path (if set)
// so status/test reflect the same DB the dispatcher would use.
func applyBrainPath(s *store.Settings) {
	if s != nil && s.Brain.DBPath != "" {
		brain.SetDBPath(s.Brain.DBPath)
	}
}

// brainStatusHandler — GET /api/brain/status
func brainStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d, err := store.Open()
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s, _ := store.LoadSettings(d)
	applyBrainPath(s)
	writeJSON(w, http.StatusOK, brain.GetStats(r.Context()))
}

// brainConfigHandler — GET/PUT /api/brain/config
func brainConfigHandler(w http.ResponseWriter, r *http.Request) {
	d, err := store.Open()
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s, err := store.LoadSettings(d)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, s.Brain)
	case http.MethodPut, http.MethodPatch:
		var cfg store.BrainConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
			return
		}
		s, err := store.LoadSettings(d)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.Brain = cfg
		if err := store.SaveSettings(d, s); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		applyBrainPath(s)
		writeJSON(w, http.StatusOK, s.Brain)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// brainExploreHandler — GET /api/brain/explore
// Content overview of the knowledge brain (counts + breakdowns), mirroring the
// flowork FQ-Brain Explorer overview.
func brainExploreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d, _ := store.Open()
	s, _ := store.LoadSettings(d)
	applyBrainPath(s)
	writeJSON(w, http.StatusOK, brain.Explore(r.Context()))
}

// brainConstitutionHandler — sacred rules from the knowledge brain.
//
//	GET    /api/brain/constitution[?limit=N]  — list
//	POST   /api/brain/constitution            — add {section,content,amplitude,source}
//	PUT    /api/brain/constitution            — update {id,content,amplitude}
//	DELETE /api/brain/constitution?id=N        — soft-delete (tombstone)
//
// Writes make flow_router the sole brain owner (option C); deletes are
// tombstones (never hard DROP), honoring the brain's append-only doctrine.
func brainConstitutionHandler(w http.ResponseWriter, r *http.Request) {
	d, _ := store.Open()
	s, _ := store.LoadSettings(d)
	applyBrainPath(s)
	if !brain.Available() {
		writeJSON(w, http.StatusOK, map[string]any{"available": false, "entries": []any{}})
		return
	}
	switch r.Method {
	case http.MethodGet:
		limit := atoiDefault(r.URL.Query().Get("limit"), 100)
		entries, err := brain.ListConstitution(r.Context(), limit, 1200)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"available": true, "entries": entries})
	case http.MethodPost:
		var b struct {
			Section   string  `json:"section"`
			Content   string  `json:"content"`
			Amplitude float64 `json:"amplitude"`
			Source    string  `json:"source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
			return
		}
		id, err := brain.AddConstitution(r.Context(), b.Section, b.Content, b.Amplitude, b.Source)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	case http.MethodPut:
		var b struct {
			ID        int64   `json:"id"`
			Content   string  `json:"content"`
			Amplitude float64 `json:"amplitude"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := brain.UpdateConstitution(r.Context(), b.ID, b.Content, b.Amplitude); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case http.MethodDelete:
		id := int64(atoiDefault(r.URL.Query().Get("id"), 0))
		if id == 0 {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		if err := brain.SoftDeleteConstitution(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// brainContributionsHandler — GET /api/brain/contributions[?pending=1&limit=N]
// Lists queued interactions + total/pending counts for the compounding loop.
func brainContributionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d, err := store.Open()
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pending := r.URL.Query().Get("pending") == "1"
	limit := atoiDefault(r.URL.Query().Get("limit"), 200)
	list, _ := store.ListBrainContributions(d, pending, limit)
	total, pend := store.CountBrainContributions(d)
	writeJSON(w, http.StatusOK, map[string]any{
		"total": total, "pending": pend, "contributions": list,
	})
}

// brainContributionsIngestHandler — POST /api/brain/contributions/ingest
// {"maxId":N} marks contributions up to maxId as ingested (called by whatever
// consumes the queue into the master brain).
func brainContributionsIngestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		MaxID int64 `json:"maxId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
		return
	}
	d, _ := store.Open()
	n, err := store.MarkContributionsIngested(d, body.MaxID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"marked": n})
}

// atoiDefault parses an int, returning def on failure/empty.
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// brainTestHandler — POST /api/brain/test {"query":"...","wings":[...],"topK":n}
// Previews exactly what enrichment would inject: retrieved snippets + skills.
func brainTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Query string   `json:"query"`
		Wings []string `json:"wings,omitempty"`
		TopK  int      `json:"topK,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Query == "" {
		http.Error(w, "query required", http.StatusBadRequest)
		return
	}
	d, _ := store.Open()
	s, _ := store.LoadSettings(d)
	applyBrainPath(s)
	if !brain.Available() {
		writeJSON(w, http.StatusOK, map[string]any{
			"available": false, "path": brain.DBPath(),
			"snippets": []any{}, "skills": []any{},
		})
		return
	}
	db, err := brain.Open()
	if err != nil {
		http.Error(w, "brain open: "+err.Error(), http.StatusInternalServerError)
		return
	}
	topK := body.TopK
	if topK <= 0 {
		topK = 5
	}
	snips, _ := brain.Retrieve(r.Context(), db, body.Query, brain.RetrieveOpts{
		Limit: topK, Wings: body.Wings, MaxContentLen: 400,
	})
	skills := brain.SelectSkills(body.Query, 3)
	writeJSON(w, http.StatusOK, map[string]any{
		"available": true,
		"query":     body.Query,
		"snippets":  snips,
		"skills":    skills,
	})
}
