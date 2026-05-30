// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider adapter.

// TTS provider catalog (10 vendor).

package tts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Request is the minimum TTS shape. Voice is vendor-specific id (e.g. "alloy"
// for OpenAI, "en-US-Standard-A" for Google, custom for ElevenLabs).
type Request struct {
	Model          string
	Input          string
	Voice          string
	ResponseFormat string // mp3 / wav / opus / flac / pcm
	Speed          float64
	APIKey         string
	BaseURL        string
	Extra          map[string]any
}

// TTSProvider is the vendor contract. Speak returns the raw audio bytes plus
// the actual content-type so the dispatcher can write it back to the client.
type TTSProvider interface {
	Name() string
	Speak(ctx context.Context, req Request) (audio []byte, contentType string, err error)
}

var (
	regMu    sync.RWMutex
	registry = map[string]TTSProvider{}
)

// Register adds a provider.
func Register(p TTSProvider) {
	if p == nil || p.Name() == "" {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	registry[p.Name()] = p
}

// Get returns a provider by name, or nil.
func Get(name string) TTSProvider {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[name]
}

// List returns every registered name.
func List() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

// ── shared helpers ────────────────────────────────────────────────────────

var ttsHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// doAudioRequest sends r and returns (body, contentType, err). The body is
// streamed up to 64 MiB.
func doAudioRequest(r *http.Request) ([]byte, string, error) {
	resp, err := ttsHTTPClient.Do(r)
	if err != nil {
		return nil, "", fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("upstream %d: %s", resp.StatusCode, head(body))
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func head(b []byte) string {
	if len(b) > 240 {
		return string(b[:240]) + "…"
	}
	return string(b)
}
