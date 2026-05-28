// Vendor: openai-compat — for any OpenAI-compatible embedding endpoint
// (Together, DeepInfra, Cohere-OpenAI-shim, local OpenAI-compat servers).
// Uses the same wire shape as openai; vendor's baseURL is what differs.
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func init() { Register(&openaiCompatProvider{}) }

type openaiCompatProvider struct{}

func (o *openaiCompatProvider) Name() string { return "openaiCompat" }

func (o *openaiCompatProvider) Embed(ctx context.Context, req Request) (*Result, error) {
	if req.BaseURL == "" {
		// Without a custom base this collapses into the regular openai vendor.
		req.BaseURL = "https://api.openai.com/v1"
	}
	payload := map[string]any{
		"model": req.Model,
		"input": req.Input,
	}
	if req.Dimensions > 0 {
		payload["dimensions"] = req.Dimensions
	}
	body, _ := json.Marshal(payload)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, req.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	return doEmbedRequest(r)
}
