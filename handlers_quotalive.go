// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — HTTP handler.

// Live quota dispatch.

package main

import (
	"context"
	"net/http"
	"time"

	"github.com/flowork-os/flowork_Router/internal/creds"
	"github.com/flowork-os/flowork_Router/internal/quotalive"
)

func quotaLiveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     "provider query param required",
			"supported": quotalive.List(),
		})
		return
	}
	impl := quotalive.Get(provider)
	if impl == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error":     "no live fetcher registered for " + provider,
			"supported": quotalive.List(),
		})
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		var err error
		token, err = resolveLiveToken(provider)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":    err.Error(),
				"hint":     "pass ?token=<value> or configure the provider's credentials",
				"provider": provider,
			})
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	snap, err := impl.Fetch(ctx, quotalive.Params{Token: token})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":    err.Error(),
			"provider": provider,
		})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// resolveLiveToken auto-loads the upstream credential from the same on-disk
// sources the dispatcher uses for subscription auth. The Copilot fetcher
// has no on-disk default in this build — callers must pass ?token=.
func resolveLiveToken(provider string) (string, error) {
	switch provider {
	case "claude":
		c, err := creds.Load()
		if err != nil {
			return "", err
		}
		if c.IsExpired() {
			return "", errExpiredClaude
		}
		return c.ClaudeAiOauth.AccessToken, nil
	default:
		return "", errNoAutoToken(provider)
	}
}

type tokenErr struct{ msg string }

func (e tokenErr) Error() string { return e.msg }

var errExpiredClaude = tokenErr{msg: "claude oauth token expired — re-login in Claude Code"}

func errNoAutoToken(p string) error {
	return tokenErr{msg: "no auto-token loader for " + p + " in this build; pass ?token=<value>"}
}
