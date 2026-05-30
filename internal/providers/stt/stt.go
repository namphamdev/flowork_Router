// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider adapter.

// STT provider catalog.

package stt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Request is the minimum transcription shape. Audio is raw bytes; AudioMIME
// is the source content-type (audio/wav, audio/mp3, …). Language is BCP-47
// ("en", "id", "ja") or empty for auto-detect.
type Request struct {
	Model     string
	Audio     []byte
	AudioMIME string
	Language  string
	FileName  string // optional, used by vendors that switch on extension
	APIKey    string
	BaseURL   string
	Extra     map[string]any
}

// Result is the vendor-neutral transcription response. Text is the plain
// transcript; ResponseJSON is the raw upstream JSON for clients that want
// segments/words/confidence.
type Result struct {
	Text         string
	Language     string
	DurationSec  float64
	ResponseJSON []byte // raw upstream JSON; pass-through for "verbose_json" callers
}

// STTProvider is the vendor contract.
type STTProvider interface {
	Name() string
	Transcribe(ctx context.Context, req Request) (Result, error)
}

var (
	regMu    sync.RWMutex
	registry = map[string]STTProvider{}
)

// Register adds a provider (idempotent — last writer wins).
func Register(p STTProvider) {
	if p == nil || p.Name() == "" {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	registry[p.Name()] = p
}

// Get returns the provider by name, or nil.
func Get(name string) STTProvider {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[name]
}

// List returns every registered vendor name (no ordering guarantee).
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

var sttHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// doJSONRequest sends r and returns (body, err). Body is streamed up to 64
// MiB to bound transcript size from misbehaving upstreams.
func doJSONRequest(r *http.Request) ([]byte, error) {
	resp, err := sttHTTPClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, head(body))
	}
	return body, nil
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

// resolveAudioMIME normalises the audio content-type. If req.AudioMIME is
// already an audio/* type we keep it. Otherwise we infer from FileName ext.
// Defaults to application/octet-stream so vendors that sniff still work.
func resolveAudioMIME(req Request) string {
	if len(req.AudioMIME) >= 6 && req.AudioMIME[:6] == "audio/" {
		return req.AudioMIME
	}
	if req.FileName == "" {
		return defaultStr(req.AudioMIME, "application/octet-stream")
	}
	// inline ext scan (avoid importing strings/filepath for a 6-byte tail)
	n := len(req.FileName)
	dot := -1
	for i := n - 1; i >= 0 && i > n-7; i-- {
		if req.FileName[i] == '.' {
			dot = i
			break
		}
	}
	if dot < 0 {
		return defaultStr(req.AudioMIME, "application/octet-stream")
	}
	switch req.FileName[dot+1:] {
	case "mp3":
		return "audio/mpeg"
	case "mp4", "m4a":
		return "audio/mp4"
	case "wav":
		return "audio/wav"
	case "ogg":
		return "audio/ogg"
	case "flac":
		return "audio/flac"
	case "webm":
		return "audio/webm"
	case "aac":
		return "audio/aac"
	case "opus":
		return "audio/opus"
	}
	return defaultStr(req.AudioMIME, "application/octet-stream")
}
