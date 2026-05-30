// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/providers/tts package — audit pass surface review.

// Vendor: openai — TTS-1 / TTS-1-HD.
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func init() { Register(&openaiProvider{}) }

type openaiProvider struct{}

func (o *openaiProvider) Name() string { return "openai" }

func (o *openaiProvider) Speak(ctx context.Context, req Request) ([]byte, string, error) {
	base := req.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	body, _ := json.Marshal(map[string]any{
		"model":           defaultStr(req.Model, "tts-1"),
		"input":           req.Input,
		"voice":           defaultStr(req.Voice, "alloy"),
		"response_format": defaultStr(req.ResponseFormat, "mp3"),
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	r.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	return doAudioRequest(r)
}
