// Executor: cursor — Cursor IDE backend (api2.cursor.sh with session + checksum).
package executors

import (
	"context"
	"fmt"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/store"
)

func init() {
	Register(&cursorExecutor{name: "cursor"})
	Register(&cursorExecutor{name: "cu"}) // alias matching upstream index.js
}

type cursorExecutor struct{ name string }

func (c *cursorExecutor) Name() string { return c.name }

func (c *cursorExecutor) endpoint(p *store.ProviderConnection) string {
	base := ProviderString(p, store.CfgBaseURL)
	if base == "" {
		base = "https://api2.cursor.sh"
	}
	return trimRightSlash(base) + "/aiserver.v1.ChatService/StreamChat"
}

func (c *cursorExecutor) headers(p *store.ProviderConnection) map[string]string {
	h := map[string]string{
		"Accept":            "*/*",
		"x-ghost-mode":      "false",
		"x-client-key":      "upstream",
	}
	if tok, ok := p.Data[store.CfgAPIKey].(string); ok && tok != "" {
		// Cursor sends the session token via the Cookie header — bearer also
		// works for the OpenAI-compat passthrough endpoint when configured.
		h["Authorization"] = "Bearer " + tok
	}
	if checksum, ok := p.Data["cursorChecksum"].(string); ok && checksum != "" {
		h["x-cursor-checksum"] = checksum
	}
	if sid, ok := p.Data["sessionId"].(string); ok && sid != "" {
		h["x-cursor-session-id"] = sid
	}
	return h
}

func (c *cursorExecutor) Stream(ctx context.Context, p *store.ProviderConnection, req Request, w http.ResponseWriter, flusher http.Flusher) (Usage, int, error) {
	body := MarshalRequest(req)
	httpReq, err := BuildRequest(ctx, http.MethodPost, c.endpoint(p), body, c.headers(p))
	if err != nil {
		return Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoStream(httpReq, w, flusher)
}

func (c *cursorExecutor) NonStream(ctx context.Context, p *store.ProviderConnection, req Request) ([]byte, Usage, int, error) {
	req.Stream = false
	body := MarshalRequest(req)
	httpReq, err := BuildRequest(ctx, http.MethodPost, c.endpoint(p), body, c.headers(p))
	if err != nil {
		return nil, Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoNonStream(httpReq)
}
