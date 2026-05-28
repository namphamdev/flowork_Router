// Locale + Init + Shutdown + Version HTTP Handlers.

package main

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/flowork-os/flowork_Router/internal/i18n"
	"github.com/flowork-os/flowork_Router/internal/store"
)

// localeCatalogHandler — GET ?tag=<lang> returns the embedded translation
// map for that locale (falls back to "en" when tag unknown). Used by the
// dashboard's locale switcher to actually swap visible text at runtime.
func localeCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = "en"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tag":           tag,
		"availableTags": i18n.AvailableTags(),
		"strings":       i18n.Catalog(tag),
	})
}

// localeHandler — GET load / PUT save.
func localeHandler(w http.ResponseWriter, r *http.Request) {
	d, _ := store.Open()
	switch r.Method {
	case http.MethodGet:
		p, err := store.LoadLocalePref(d)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, p)
	case http.MethodPut, http.MethodPatch:
		var p store.LocalePref
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := store.SaveLocalePref(d, &p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, p)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// initHandler — GET return bootstrap snapshot for the dashboard SPA.
// Pulls settings + locale + providers count + version + flags.
func initHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d, _ := store.Open()
	settings, _ := store.LoadSettings(d)
	if settings != nil {
		settings.Password = ""
	}
	locale, _ := store.LoadLocalePref(d)
	providers, _ := store.ListProviders(d)
	tunnel, _ := store.LoadTunnelState(d)
	writeJSON(w, http.StatusOK, map[string]any{
		"version":       version,
		"runtime":       runtime.Version(),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"settings":      settings,
		"locale":        locale,
		"providerCount": len(providers),
		"tunnel":        tunnel,
		"startedAt":     processStartedAt.Format(time.RFC3339),
		"now":           time.Now().UTC().Format(time.RFC3339),
	})
}

// shutdownHandler — POST gracefully stop the router. Requires admin auth in
// future; for now Phase 1 accepts unauthenticated (single-user local).
func shutdownHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shuttingDown": true})
	go func() {
		time.Sleep(200 * time.Millisecond)
		if shutdownTriggerCh != nil {
			shutdownTriggerCh <- struct{}{}
		}
	}()
}

// versionHandler — GET current build version + uptime.
func versionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    version,
		"runtime":    runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"startedAt":  processStartedAt.Format(time.RFC3339),
		"uptimeSec":  int64(time.Since(processStartedAt).Seconds()),
		"updateChan": "stable",
	})
}

// versionUpdateHandler — POST trigger self-update. Phase 1 stub: returns
// "no-op" since flow_router updates by re-running build.sh manually.
func versionUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "noop",
		"message": "flow_router updates via rebuild — run start.sh / go build manually",
		"phase":   "phase2_pending",
	})
}

// versionShutdownHandler — POST /api/version/shutdown alias for shutdown.
func versionShutdownHandler(w http.ResponseWriter, r *http.Request) {
	shutdownHandler(w, r)
}
