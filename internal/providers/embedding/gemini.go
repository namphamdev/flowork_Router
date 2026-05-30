// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider adapter.

// Vendor: gemini — text-embedding-004 / -005 via generativelanguage API.
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func init() { Register(&geminiProvider{}) }

type geminiProvider struct{}

func (g *geminiProvider) Name() string { return "gemini" }

// Embed adapts the Gemini batch-embed endpoint to the OpenAI Result shape.
func (g *geminiProvider) Embed(ctx context.Context, req Request) (*Result, error) {
	base := req.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	model := defaultStr(req.Model, "text-embedding-004")
	url := base + "/models/" + model + ":batchEmbedContents?key=" + req.APIKey

	requests := make([]map[string]any, len(req.Input))
	for i, text := range req.Input {
		entry := map[string]any{
			"model": "models/" + model,
			"content": map[string]any{
				"parts": []map[string]any{{"text": text}},
			},
		}
		requests[i] = entry
	}
	body, _ := json.Marshal(map[string]any{"requests": requests})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := embedHTTPClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, head(raw))
	}
	var gemResp struct {
		Embeddings []struct {
			Values []float64 `json:"values"`
		} `json:"embeddings"`
	}
	if err := json.Unmarshal(raw, &gemResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out := &Result{Object: "list", Model: model}
	for i, e := range gemResp.Embeddings {
		out.Data = append(out.Data, Embed{Object: "embedding", Embedding: e.Values, Index: i})
	}
	return out, nil
}
