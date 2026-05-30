// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/providers/image package — audit pass surface review.

// Vendor: gemini — Imagen 3 via generativelanguage.googleapis.com.
package image

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func init() { Register(&geminiImageProvider{}) }

type geminiImageProvider struct{}

func (g *geminiImageProvider) Name() string { return "gemini" }

func (g *geminiImageProvider) Generate(ctx context.Context, req Request) (*Result, error) {
	base := req.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	model := defaultStr(req.Model, "imagen-3.0-generate-001")
	url := base + "/models/" + model + ":predict?key=" + req.APIKey
	body, _ := json.Marshal(map[string]any{
		"instances": []map[string]any{{"prompt": req.Prompt}},
		"parameters": map[string]any{
			"sampleCount": defaultInt(req.N, 1),
		},
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	return doImageRequest(r)
}
