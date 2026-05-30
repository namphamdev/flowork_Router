// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Vendor: tavily — Tavily search API.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func init() { Register(&tavilyProvider{}) }

type tavilyProvider struct{}

func (t *tavilyProvider) Name() string { return "tavily" }

func (t *tavilyProvider) Search(ctx context.Context, req Request) (*Result, error) {
	base := req.BaseURL
	if base == "" {
		base = "https://api.tavily.com"
	}
	body, _ := json.Marshal(map[string]any{
		"api_key":     req.APIKey,
		"query":       req.Query,
		"max_results": defaultInt(req.MaxResults, 5),
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	var resp struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := doRequest(r, &resp); err != nil {
		return nil, err
	}
	out := &Result{Provider: "tavily", Query: req.Query}
	for _, x := range resp.Results {
		out.Results = append(out.Results, SearchResult{Title: x.Title, URL: x.URL, Snippet: x.Content})
	}
	return out, nil
}
