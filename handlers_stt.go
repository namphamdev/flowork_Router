// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — HTTP handler.

// STT dispatch handler.

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flowork-os/flowork_Router/internal/providers/stt"
	"github.com/flowork-os/flowork_Router/internal/store"
)

// transcriptionsHandler — POST /v1/audio/transcriptions (and /translations).
// Reads multipart/form-data, dispatches to the active STT MediaProvider's
// vendor implementation, and returns OpenAI-compat JSON.
func transcriptionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 32 MiB cap — same envelope as dispatchMedia's body limit.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing 'file' field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	audio, err := io.ReadAll(io.LimitReader(file, 32<<20))
	if err != nil {
		http.Error(w, "read file: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Look up active STT provider.
	d, _ := store.Open()
	providers, err := store.ListMediaProviders(d, store.MediaCategorySTT)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var picked *store.MediaProvider
	for i := range providers {
		if providers[i].IsActive {
			picked = &providers[i]
			break
		}
	}
	if picked == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"category": store.MediaCategorySTT,
			"message":  "no active STT provider — add one in Media Providers (deepgram / assemblyai / gemini / openai)",
		})
		return
	}

	impl := stt.Get(picked.Provider)
	if impl == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message":   fmt.Sprintf("provider %q is not implemented in this build", picked.Provider),
			"supported": stt.List(),
		})
		return
	}

	mime := ""
	filename := ""
	if header != nil {
		mime = header.Header.Get("Content-Type")
		filename = header.Filename
	}

	// AssemblyAI's poll loop is 120s; allow margin for upload + submit.
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	req := stt.Request{
		Model:     pickFormValue(r, "model", picked.Models),
		Audio:     audio,
		AudioMIME: mime,
		Language:  r.FormValue("language"),
		FileName:  filename,
		APIKey:    picked.APIKey,
		BaseURL:   picked.BaseURL,
	}
	res, err := impl.Transcribe(ctx, req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":    err.Error(),
			"provider": picked.Provider,
		})
		return
	}

	// Honour `response_format` like OpenAI: "text" → plain text; "verbose_json"
	// → upstream raw passthrough when available; anything else → JSON {text}.
	switch strings.ToLower(r.FormValue("response_format")) {
	case "text":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, res.Text)
	case "verbose_json":
		if len(res.ResponseJSON) > 0 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(res.ResponseJSON)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"text":     res.Text,
			"language": res.Language,
			"duration": res.DurationSec,
		})
	default:
		out := map[string]any{"text": res.Text}
		if res.Language != "" {
			out["language"] = res.Language
		}
		if res.DurationSec > 0 {
			out["duration"] = res.DurationSec
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// pickFormValue returns the form value if set; otherwise the first item of
// fallback (or "" if fallback is empty). Used to seed model from provider
// defaults when the OpenAI-compat client doesn't pass one.
func pickFormValue(r *http.Request, name string, fallback []string) string {
	if v := r.FormValue(name); v != "" {
		return v
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}
