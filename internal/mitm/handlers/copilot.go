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
