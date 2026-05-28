// Skill Invocation (/v1/skills/:name).

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flowork-os/flowork_Router/internal/router"
	"github.com/flowork-os/flowork_Router/internal/store"
)

// skillInvokeHandler — GET /v1/skills/ lists skills; POST /v1/skills/:name runs one.
func skillInvokeHandler(w http.ResponseWriter, r *http.Request) {
	d, _ := store.Open()
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/skills/"), "/")

	if r.Method == http.MethodGet {
		skills, err := store.ListSkills(d)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out := make([]map[string]any, 0, len(skills))
		for _, s := range skills {
			out = append(out, map[string]any{
				"name": s.Name, "description": s.Description,
				"variables": s.Variables, "model": s.DefaultModel,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out, "count": len(out)})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if name == "" {
		writeSkillError(w, http.StatusBadRequest, "skill name required: POST /v1/skills/<name>")
		return
	}

	skill, err := store.GetSkillByName(d, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if skill == nil {
		writeSkillError(w, http.StatusNotFound, "no skill named "+name)
		return
	}

	var body struct {
		Variables   map[string]string `json:"variables"`
		Model       string            `json:"model"`
		Stream      bool              `json:"stream"`
		Temperature *float64          `json:"temperature"`
		MaxTokens   int               `json:"max_tokens"`
	}
	// Empty body is valid (skill may have no variables).
	if raw, _ := io.ReadAll(io.LimitReader(r.Body, 1*1024*1024)); len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			writeSkillError(w, http.StatusBadRequest, "parse: "+err.Error())
			return
		}
	}

	model := body.Model
	if model == "" {
		model = skill.DefaultModel
	}
	if model == "" {
		writeSkillError(w, http.StatusBadRequest, "skill has no defaultModel — pass \"model\" in the request")
		return
	}

	temperature := skill.Temperature
	if body.Temperature != nil {
		temperature = *body.Temperature
	}
	maxTokens := skill.MaxTokens
	if body.MaxTokens > 0 {
		maxTokens = body.MaxTokens
	}

	var msgs []router.OpenAIMessage
	if strings.TrimSpace(skill.SystemPrompt) != "" {
		msgs = append(msgs, router.OpenAIMessage{Role: "system", Content: skill.SystemPrompt})
	}
	msgs = append(msgs, router.OpenAIMessage{
		Role:    "user",
		Content: store.RenderSkillTemplate(skill.UserTemplate, body.Variables),
	})

	req := router.OpenAIRequest{
		Model:       model,
		Messages:    msgs,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      body.Stream,
	}

	if req.Stream {
		status, _, derr := router.DispatchChatCompletionStream(r.Context(), req, w)
		if derr != nil && status != http.StatusOK {
			writeJSON(w, status, map[string]any{
				"error": map[string]any{"type": "router_error", "message": derr.Error()},
			})
		}
		return
	}

	start := time.Now()
	resp, status, derr := router.DispatchChatCompletion(r.Context(), req)
	durationMs := time.Since(start).Milliseconds()
	if derr != nil {
		errBody := map[string]any{"error": map[string]any{"type": "router_error", "message": derr.Error()}}
		raw, _ := json.Marshal(errBody)
		captureMITM(model, r, []byte("skill:"+name), status, derr.Error(), durationMs, raw)
		writeJSON(w, status, errBody)
		return
	}
	respBody, _ := json.Marshal(resp)
	captureMITM(resp.Model, r, []byte("skill:"+name), status, "", durationMs, respBody)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(respBody)
}

func writeSkillError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"type": "skill_error", "message": msg},
	})
}
