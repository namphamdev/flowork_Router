// Vendor: raw — plain HTTP GET, no readability/cleanup.
// Returns the upstream body bytes as-is with its original Content-Type.
// Useful when the agent wants the actual HTML / JSON / text and will
// extract on its own.
package fetch

import (
	"context"
	"fmt"
	"net/http"
)

func init() { Register(&rawProvider{}) }

type rawProvider struct{}

func (r *rawProvider) Name() string { return "raw" }

func (r *rawProvider) Fetch(ctx context.Context, req Request) (Result, error) {
	if req.URL == "" {
		return Result{}, fmt.Errorf("raw: url required")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build req: %w", err)
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; flow_router/1.0)")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	body, resp, err := doHTTPRequest(httpReq)
	if err != nil {
		return Result{}, err
	}
	return Result{
		URL:         req.URL,
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
	}, nil
}
