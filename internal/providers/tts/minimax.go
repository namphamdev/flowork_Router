// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider adapter (LLM/TTS/embedding).

// Vendor: minimax — MiniMax Speech (T2A v2).
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func init() { Register(&minimaxProvider{}) }

type minimaxProvider struct{}

func (m *minimaxProvider) Name() string { return "minimax" }

func (m *minimaxProvider) Speak(ctx context.Context, req Request) ([]byte, string, error) {
	base := req.BaseURL
	if base == "" {
		base = "https://api.minimax.chat/v1"
	}
	body, _ := json.Marshal(map[string]any{
		"model": defaultStr(req.Model, "speech-01"),
		"text":  req.Input,
		"voice_setting": map[string]any{
			"voice_id": defaultStr(req.Voice, "female-shaonv"),
			"speed":    1.0,
			"vol":      1.0,
			"pitch":    0,
		},
		"audio_setting": map[string]any{
			"sample_rate": 32000,
			"bitrate":     128000,
			"format":      defaultStr(req.ResponseFormat, "mp3"),
		},
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/t2a_v2", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	r.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	return doAudioRequest(r)
}
