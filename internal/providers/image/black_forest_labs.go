// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider adapter (LLM/TTS/embedding).

// Vendor: blackForestLabs — Flux.1.1 / FLUX pro via api.bfl.ml.
package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func init() { Register(&bflProvider{}) }

type bflProvider struct{}

func (b *bflProvider) Name() string { return "blackForestLabs" }

func (b *bflProvider) Generate(ctx context.Context, req Request) (*Result, error) {
	base := req.BaseURL
	if base == "" {
		base = "https://api.bfl.ml/v1"
	}
	w, h := splitSize(req.Size, 1024)
	body, _ := json.Marshal(map[string]any{
		"prompt": req.Prompt,
		"width":  w,
		"height": h,
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/"+defaultStr(req.Model, "flux-pro"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		r.Header.Set("x-key", req.APIKey)
	}
	return doImageRequest(r)
}

// splitSize parses "WxH" → (w, h); defaults both to fallback when malformed.
func splitSize(s string, fallback int) (int, int) {
	if s == "" {
		return fallback, fallback
	}
	var w, h int
	if _, err := fmt.Sscanf(s, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
		return fallback, fallback
	}
	return w, h
}
