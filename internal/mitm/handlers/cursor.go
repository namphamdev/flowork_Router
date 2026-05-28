// Per-IDE MITM handler: cursor (Cursor IDE chat backend).
package handlers

import "net/http"

func init() { Register(&cursorHandler{}) }

type cursorHandler struct{}

func (c *cursorHandler) Name() string { return "cursor" }

// Handle maps Cursor's /BidiAppend, /RunSSE, /RunPoll, /Run RPC paths to
// flow_router's chat completions endpoint. We strip the Cursor checksum +
// session headers — the local dispatcher does not need them and they only
// carry IDE-specific telemetry.
func (c *cursorHandler) Handle(w http.ResponseWriter, r *http.Request) {
	r.Header.Del("x-cursor-checksum")
	r.Header.Del("x-cursor-session-id")
	r.Header.Del("x-ghost-mode")
	rerouteToRouter(w, r, "/v1/chat/completions")
}
