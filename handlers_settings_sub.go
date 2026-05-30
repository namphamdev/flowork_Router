// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — HTTP handler.

// Settings Sub-routes (database / proxy-test /.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// settingsDatabaseHandler — GET DB stats (file size, table row counts).
func settingsDatabaseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d, err := store.Open()
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tables := []string{
		"providerConnections", "providerNodes", "apiKeys", "usageDaily",
		"usageHistory", "requestDetails", "combos", "proxyPools", "kv",
		"tags", "pricing", "modelAlias", "modelAvailability",
		"authSessions", "translatorDrafts", "modelsCustom", "modelsDisabled",
	}
	counts := map[string]int{}
	for _, t := range tables {
		var n int
		_ = d.QueryRow("SELECT COUNT(*) FROM " + t).Scan(&n)
		counts[t] = n
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dbPath": store.DBPath(),
		"counts": counts,
	})
}

// settingsProxyTestHandler — POST { url } test outbound through proxy.
// Returns { reachable, latencyMs, statusCode, error }.
func settingsProxyTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		URL       string `json:"url"`
		ProxyURL  string `json:"proxyUrl,omitempty"`
		TimeoutMs int    `json:"timeoutMs,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.URL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}
	timeout := time.Duration(body.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	transport := &http.Transport{}
	if body.ProxyURL != "" {
		u, err := url.Parse(body.ProxyURL)
		if err != nil {
			http.Error(w, "bad proxyUrl: "+err.Error(), http.StatusBadRequest)
			return
		}
		transport.Proxy = http.ProxyURL(u)
	}
	client := &http.Client{Transport: transport, Timeout: timeout}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", body.URL, nil)
	if err != nil {
		http.Error(w, "request: "+err.Error(), http.StatusBadRequest)
		return
	}
	t0 := time.Now()
	resp, err := client.Do(req)
	dur := time.Since(t0)
	out := map[string]any{
		"latencyMs": dur.Milliseconds(),
	}
	if err != nil {
		out["reachable"] = false
		out["error"] = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	defer resp.Body.Close()
	out["reachable"] = true
	out["statusCode"] = resp.StatusCode
	writeJSON(w, http.StatusOK, out)
}

// settingsRequireLoginHandler — GET / PUT toggle the requireLogin flag and
// (when enabling password mode) set the admin password. Body for PUT:
// { requireLogin: bool, authMode: "password|oidc|none", password?: string }.
func settingsRequireLoginHandler(w http.ResponseWriter, r *http.Request) {
	d, _ := store.Open()
	settings, err := store.LoadSettings(d)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"requireLogin":   settings.RequireLogin,
			"authMode":       settings.AuthMode,
			"passwordSet":    settings.Password != "",
			"oidcConfigured": len(settings.OidcConfig) > 0,
		})
	case http.MethodPut:
		var body struct {
			RequireLogin *bool          `json:"requireLogin"`
			AuthMode     string         `json:"authMode"`
			Password     string         `json:"password,omitempty"`
			OidcConfig   map[string]any `json:"oidcConfig,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.RequireLogin != nil {
			settings.RequireLogin = *body.RequireLogin
		}
		if body.AuthMode != "" {
			settings.AuthMode = body.AuthMode
		}
		if body.Password != "" {
			settings.Password = hashPassword(body.Password)
		}
		if body.OidcConfig != nil {
			settings.OidcConfig = body.OidcConfig
		}
		if err := store.SaveSettings(d, settings); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"requireLogin": settings.RequireLogin,
			"authMode":     settings.AuthMode,
			"passwordSet":  settings.Password != "",
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
