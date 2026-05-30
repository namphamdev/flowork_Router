// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/mitm/handlers package — audit pass surface review.

// Per-IDE MITM handler: copilot (GitHub Copilot Chat).
package handlers

import "net/http"

func init() { Register(&copilotHandler{}) }

type copilotHandler struct{}

func (c *copilotHandler) Name() string { return "copilot" }

// Handle reroutes /chat/completions, /v1/messages, /responses to flow_router.
func (c *copilotHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Strip Copilot-specific headers that confuse the local dispatcher.
	r.Header.Del("editor-version")
	r.Header.Del("editor-plugin-version")
	r.Header.Del("copilot-integration-id")
	rerouteToRouter(w, r, r.URL.Path)
}
