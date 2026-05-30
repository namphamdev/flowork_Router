// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/providers/image package — audit pass surface review.

// Vendor: nanobanana — Nano Banana image-gen API.
package image

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func init() { Register(&nanobananaProvider{}) }

type nanobananaProvider struct{}

func (n *nanobananaProvider) Name() string { return "nanobanana" }

func (n *nanobananaProvider) Generate(ctx context.Context, req Request) (*Result, error) {
	base := req.BaseURL
	if base == "" {
		base = "https://api.nanobanana.io/v1"
	}
	body, _ := json.Marshal(map[string]any{
		"prompt":         req.Prompt,
		"negativePrompt": req.NegativePrompt,
		"size":           defaultStr(req.Size, "1024x1024"),
		"count":          defaultInt(req.N, 1),
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/images/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	return doImageRequest(r)
}
