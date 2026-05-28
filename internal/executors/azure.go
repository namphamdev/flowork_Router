// Executor: azure — Azure OpenAI deployment endpoint shape.
package executors

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/flowork-os/flowork_Router/internal/store"
)

func init() { Register(&azureExecutor{}) }

type azureExecutor struct{}

func (a *azureExecutor) Name() string { return "azure" }

// endpoint composes the Azure URL:
//
//	{baseURL}/openai/deployments/{deployment}/chat/completions?api-version=...
//
// deployment defaults to the model name when the provider's `deployment` config
// is empty.
func (a *azureExecutor) endpoint(p *store.ProviderConnection, model string) string {
	base := ProviderString(p, store.CfgBaseURL)
	deployment, _ := p.Data["deployment"].(string)
	if deployment == "" {
		deployment = model
	}
	apiVersion, _ := p.Data["apiVersion"].(string)
	if apiVersion == "" {
		apiVersion = "2024-08-01-preview"
	}
	q := url.Values{}
	q.Set("api-version", apiVersion)
	return trimRightSlash(base) + "/openai/deployments/" + deployment +
		"/chat/completions?" + q.Encode()
}

func (a *azureExecutor) headers(p *store.ProviderConnection) map[string]string {
	h := map[string]string{}
	if k, ok := p.Data[store.CfgAPIKey].(string); ok && k != "" {
		h["api-key"] = k
	}
	return h
}

func (a *azureExecutor) Stream(ctx context.Context, p *store.ProviderConnection, req Request, w http.ResponseWriter, flusher http.Flusher) (Usage, int, error) {
	body := MarshalRequest(req)
	httpReq, err := BuildRequest(ctx, http.MethodPost, a.endpoint(p, req.Model), body, a.headers(p))
	if err != nil {
		return Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoStream(httpReq, w, flusher)
}

func (a *azureExecutor) NonStream(ctx context.Context, p *store.ProviderConnection, req Request) ([]byte, Usage, int, error) {
	req.Stream = false
	body := MarshalRequest(req)
	httpReq, err := BuildRequest(ctx, http.MethodPost, a.endpoint(p, req.Model), body, a.headers(p))
	if err != nil {
		return nil, Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoNonStream(httpReq)
}
