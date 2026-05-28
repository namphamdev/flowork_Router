// Vendor: edgeTts — Microsoft Edge cloud TTS via free WebSocket → HTTP shim.
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

func init() { Register(&edgeTtsProvider{}) }

type edgeTtsProvider struct{}

func (e *edgeTtsProvider) Name() string { return "edgeTts" }

// Speak uses a thin HTTP shim around the free Microsoft Edge TTS endpoint.
// The shim accepts {text, voice} and returns the rendered MP3.
func (e *edgeTtsProvider) Speak(ctx context.Context, req Request) ([]byte, string, error) {
	base := req.BaseURL
	if base == "" {
		base = "http://127.0.0.1:5050"
	}
	body, _ := json.Marshal(map[string]any{
		"text":  req.Input,
		"voice": defaultStr(req.Voice, "en-US-AriaNeural"),
		"rate":  "+0%",
	})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/tts", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	r.Header.Set("Content-Type", "application/json")
	return doAudioRequest(r)
}
