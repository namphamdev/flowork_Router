// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Vendor: jina — Jina AI Reader (r.jina.ai/{url}).
// Returns LLM-friendly markdown extraction. Free tier has rate limits;
// supplying an API key lifts them. The "url" is prefixed onto the base —
// no body, GET only.
package fetch

import (
	"context"
	"fmt"
	"net/http"
)

func init() { Register(&jinaProvider{}) }

type jinaProvider struct{}

func (j *jinaProvider) Name() string { return "jina" }

func (j *jinaProvider) Fetch(ctx context.Context, req Request) (Result, error) {
	if req.URL == "" {
		return Result{}, fmt.Errorf("jina: url required")
	}
	base := defaultStr(req.BaseURL, "https://r.jina.ai")
	endpoint := base + "/" + req.URL

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build req: %w", err)
	}
	httpReq.Header.Set("Accept", "text/markdown, text/plain, */*")
	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	body, resp, err := doHTTPRequest(httpReq)
	if err != nil {
		return Result{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("jina %d", resp.StatusCode)
	}
	return Result{
		URL:         req.URL,
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
	}, nil
}
