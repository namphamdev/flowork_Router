// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider adapter (LLM/TTS/embedding).

// Vendor: codex — ChatGPT-backend image-gen (OpenAI-shape via codex auth).
package image

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func init() { Register(&codexProvider{}) }

type codexProvider struct{}

func (c *codexProvider) Name() string { return "codex" }

func (c *codexProvider) Generate(ctx context.Context, req Request) (*Result, error) {
	base := req.BaseURL
	if base == "" {
		base = "https://chatgpt.com/backend-api"
	}
	body, _ := json.Marshal(map[string]any{
		"model":  defaultStr(req.Model, "gpt-image-1"),
		"prompt": req.Prompt,
		"n":      defaultInt(req.N, 1),
		"size":   defaultStr(req.Size, "1024x1024"),
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/codex/images", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	if acct, ok := req.Extra["chatgptAccountId"].(string); ok {
		r.Header.Set("chatgpt-account-id", acct)
	}
	return doImageRequest(r)
}
