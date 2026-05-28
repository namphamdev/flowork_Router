// Web-fetch dispatch handler.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/flowork-os/flowork_Router/internal/fetch"
	"github.com/flowork-os/flowork_Router/internal/store"
)

// webFetchHandler — POST /v1/web/fetch with JSON {url, provider?, mode?}.
// Returns either the upstream body as the response (when text/markdown/html)
// or a JSON envelope with base64-encoded bytes for non-text content.
func webFetchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		URL      string `json:"url"`
		Provider string `json:"provider,omitempty"`
		Mode     string `json:"mode,omitempty"`
		APIKey   string `json:"apiKey,omitempty"`
		BaseURL  string `json:"baseUrl,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
		return
	}
	if in.URL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}

	// Resolve which vendor to call. Explicit > active media provider > "raw" fallback.
	picked := pickFetchProvider(in.Provider, in.APIKey, in.BaseURL)

	impl := fetch.Get(picked.name)
	if impl == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     "unknown provider " + picked.name,
			"supported": fetch.List(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	res, err := impl.Fetch(ctx, fetch.Request{
		URL:     in.URL,
		Mode:    in.Mode,
		APIKey:  picked.apiKey,
		BaseURL: picked.baseURL,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":    err.Error(),
			"provider": picked.name,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"url":         res.URL,
		"title":       res.Title,
		"contentType": res.ContentType,
		"status":      res.StatusCode,
		"body":        string(res.Body),
		"provider":    picked.name,
	})
}

type fetchPick struct {
	name    string
	apiKey  string
	baseURL string
}

// pickFetchProvider resolves: explicit > active MediaProvider > "raw".
func pickFetchProvider(explicit, apiKey, baseURL string) fetchPick {
	if explicit != "" {
		return fetchPick{name: explicit, apiKey: apiKey, baseURL: baseURL}
	}
	d, _ := store.Open()
	if providers, err := store.ListMediaProviders(d, store.MediaCategoryWebFetch); err == nil {
		for i := range providers {
			if providers[i].IsActive && fetch.Get(providers[i].Provider) != nil {
				return fetchPick{
					name:    providers[i].Provider,
					apiKey:  providers[i].APIKey,
					baseURL: providers[i].BaseURL,
				}
			}
		}
	}
	return fetchPick{name: "raw"}
}
