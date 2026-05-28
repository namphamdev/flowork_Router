// Executor: antigravity — Google Cloud Code Assist Antigravity backend.
package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/cloudcode"
	"github.com/flowork-os/flowork_Router/internal/store"
)

func init() { Register(&antigravityExecutor{}) }

type antigravityExecutor struct{}

func (a *antigravityExecutor) Name() string { return "antigravity" }

func (a *antigravityExecutor) endpoint(p *store.ProviderConnection, stream bool) string {
	base := ProviderString(p, store.CfgBaseURL)
	if base == "" {
		base = "https://cloudcode-pa.googleapis.com"
	}
	action := "generateContent"
	if stream {
		action = "streamGenerateContent?alt=sse"
	}
	return trimRightSlash(base) + "/v1internal:" + action
}

func (a *antigravityExecutor) headers(p *store.ProviderConnection, stream bool) map[string]string {
	h := map[string]string{
		"User-Agent": "google-cloud-code-assist/1.16.0",
	}
	if tok, ok := p.Data[store.CfgAPIKey].(string); ok && tok != "" {
		h["Authorization"] = "Bearer " + tok
	}
	// X-Machine-Session-Id scopes Antigravity's prompt cache. The native
	// binary mints one id per launch and keeps it for the process lifetime.
	// We replicate that — stable per provider.ID within a single router
	// run, fresh on every restart — so prompt-cache continuity works.
	// Explicit sessionId on the provider record still wins.
	sid, _ := p.Data["sessionId"].(string)
	if sid == "" {
		sid = DeriveAntigravitySessionID(p.ID)
	}
	h["X-Machine-Session-Id"] = sid
	if stream {
		h["Accept"] = "text/event-stream"
	} else {
		h["Accept"] = "application/json"
	}
	return h
}

// Cloud Code Assist wraps the request in {project, model, request: <body>}.
// When the provider record sets useRealProjectId=true, we resolve a stable
// project id from cloudcode-pa (cached 1h) instead of the random id stored
// in projectId — significantly reduces the chance of Google's anti-abuse
// system flagging the connection.
func (a *antigravityExecutor) body(ctx context.Context, p *store.ProviderConnection, req Request) []byte {
	contents := make([]map[string]any, len(req.Messages))
	for i, m := range req.Messages {
		contents[i] = map[string]any{"role": m.Role, "parts": []map[string]any{{"text": m.Content}}}
	}
	project := ProviderString(p, "projectId")
	if useReal, _ := p.Data["useRealProjectId"].(bool); useReal {
		if tok, _ := p.Data[store.CfgAPIKey].(string); tok != "" {
			if real, err := cloudcode.GetProjectID(ctx, p.ID, tok); err == nil && real != "" {
				project = real
			}
		}
	}
	wrap := map[string]any{
		"project": project,
		"model":   req.Model,
		"request": map[string]any{"contents": contents},
	}
	b, _ := json.Marshal(wrap)
	return b
}

func (a *antigravityExecutor) Stream(ctx context.Context, p *store.ProviderConnection, req Request, w http.ResponseWriter, flusher http.Flusher) (Usage, int, error) {
	httpReq, err := BuildRequest(ctx, http.MethodPost, a.endpoint(p, true), a.body(ctx, p, req), a.headers(p, true))
	if err != nil {
		return Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoStream(httpReq, w, flusher)
}

func (a *antigravityExecutor) NonStream(ctx context.Context, p *store.ProviderConnection, req Request) ([]byte, Usage, int, error) {
	req.Stream = false
	httpReq, err := BuildRequest(ctx, http.MethodPost, a.endpoint(p, false), a.body(ctx, p, req), a.headers(p, false))
	if err != nil {
		return nil, Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoNonStream(httpReq)
}
