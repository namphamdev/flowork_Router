// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider executor HTTP call.

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
	tok, _ := p.Data[store.CfgAPIKey].(string)
	machineID, _ := p.Data["machineId"].(string)
	storedChecksum, _ := p.Data["cursorChecksum"].(string)
	storedSession, _ := p.Data["sessionId"].(string)

	// Auto-bundle path: when the operator gave us a token but no manually-
	// scraped checksum, derive everything Cursor's ConnectRPC endpoint
	// requires from the token + machineId (Jyh cipher on the timestamp).
	if tok != "" && storedChecksum == "" {
		h := BuildCursorHeaders(tok, machineID, false)
		h["Accept"] = "*/*"
		if storedSession != "" {
			h["x-cursor-session-id"] = storedSession
		}
		return h
	}

	// Manual-checksum path — preserved for users who pasted the value.
	h := map[string]string{
		"Accept":       "*/*",
		"x-ghost-mode": "false",
		"x-client-key": "upstream",
	}
	if tok != "" {
		h["Authorization"] = "Bearer " + tok
	}
	if storedChecksum != "" {
		h["x-cursor-checksum"] = storedChecksum
	}
	if storedSession != "" {
		h["x-cursor-session-id"] = storedSession
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
