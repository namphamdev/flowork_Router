// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 8 (Skill catalog API phase 1) DONE. Read-only
//   endpoints: GET /api/brain/skills/list (max 10 summary, anti
//   over-prompt) + /api/brain/skills/get (full detail on-demand).
//   Pure brain.Skills() passthrough — no DB write. Future contribute
//   endpoint → tambah file baru, JANGAN modify ini.
//
// handlers_brain_skills.go — Section 8 roadmap: Skill catalog API.
//
// Mr.Dev doctrine over-prompt: list endpoint return MAX 10 summary (name +
// description, NO body). Full body fetched on-demand via /get?id=.
//
// Source: flowork_Router/roadmap.md Section 8.

package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/brain"
)

// maxSkillsList — cap absolute response size (anti over-prompt). Caller
// pakai search query untuk narrow.
const maxSkillsList = 10

// SkillSummary — minimal payload untuk list response. Strip body supaya
// anti over-prompt.
type SkillSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// brainSkillsListHandler — GET /api/brain/skills/list?search=&limit=
// Return SkillSummary list (max 10). Search optional — kalau ada, ranked
// via brain.SelectSkills heuristic.
func brainSkillsListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed (use GET)", http.StatusMethodNotAllowed)
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	limit := maxSkillsList
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if n, perr := strconv.Atoi(s); perr == nil && n > 0 && n <= maxSkillsList {
			limit = n
		}
	}

	var docs []brain.SkillDoc
	if search != "" {
		docs = brain.SelectSkills(search, limit)
	} else {
		all := brain.Skills()
		if len(all) > limit {
			docs = all[:limit]
		} else {
			docs = all
		}
	}

	out := make([]SkillSummary, 0, len(docs))
	for _, d := range docs {
		out = append(out, SkillSummary{Name: d.Name, Description: d.Description})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out,
		"count": len(out),
		"total": len(brain.Skills()),
	})
}

// brainSkillsGetHandler — GET /api/brain/skills/get?id=<name>
// Return full SkillDoc (name + description + body). Caller pull on-demand
// setelah pilih dari list.
func brainSkillsGetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed (use GET)", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	// Lookup by Name. Skill embedded sorted alphabetically; linear scan OK
	// (catalog kecil — < 100 skill).
	for _, d := range brain.Skills() {
		if d.Name == id {
			writeJSON(w, http.StatusOK, d)
			return
		}
	}
	http.Error(w, "skill not found", http.StatusNotFound)
}
