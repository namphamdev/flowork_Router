// Executor: codex — ChatGPT Codex backend (chatgpt-backend with session token).
package executors

import (
	"context"
	"fmt"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/store"
)

func init() { Register(&codexExecutor{}) }

type codexExecutor struct{}

func (c *codexExecutor) Name() string { return "codex" }

func (c *codexExecutor) endpoint(p *store.ProviderConnection) string {
	base := ProviderString(p, store.CfgBaseURL)
	if base == "" {
		base = "https://chatgpt.com"
	}
	return trimRightSlash(base) + "/backend-api/codex/responses"
}

func (c *codexExecutor) headers(p *store.ProviderConnection) map[string]string {
	h := map[string]string{
		"Accept":             "text/event-stream",
		"OpenAI-Beta":        "responses=experimental",
		"originator":         "codex_cli_rs",
		"version":            "0.20.0",
	}
	if tok, ok := p.Data[store.CfgAPIKey].(string); ok && tok != "" {
		h["Authorization"] = "Bearer " + tok
	}
	if acct, ok := p.Data["chatgptAccountId"].(string); ok && acct != "" {
		h["chatgpt-account-id"] = acct
	}
	if proj, ok := p.Data["projectId"].(string); ok && proj != "" {
		h["openai-project"] = proj
	}
	return h
}

func (c *codexExecutor) Stream(ctx context.Context, p *store.ProviderConnection, req Request, w http.ResponseWriter, flusher http.Flusher) (Usage, int, error) {
	body := MarshalRequest(req)
	httpReq, err := BuildRequest(ctx, http.MethodPost, c.endpoint(p), body, c.headers(p))
	if err != nil {
		return Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoStream(httpReq, w, flusher)
}

func (c *codexExecutor) NonStream(ctx context.Context, p *store.ProviderConnection, req Request) ([]byte, Usage, int, error) {
	req.Stream = false
	body := MarshalRequest(req)
	httpReq, err := BuildRequest(ctx, http.MethodPost, c.endpoint(p), body, c.headers(p))
	if err != nil {
		return nil, Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoNonStream(httpReq)
}
