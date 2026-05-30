// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package stt

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// helper: run a single round-trip Transcribe against a fake upstream.
func runTranscribe(t *testing.T, p STTProvider, handler http.HandlerFunc, req Request) Result {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	req.BaseURL = srv.URL
	res, err := p.Transcribe(context.Background(), req)
	if err != nil {
		t.Fatalf("%s: %v", p.Name(), err)
	}
	return res
}

func TestRegistryRoundTrip(t *testing.T) {
	for _, name := range []string{"deepgram", "assemblyai", "gemini", "openai"} {
		if Get(name) == nil {
			t.Fatalf("provider %q not registered", name)
		}
	}
	if Get("nonexistent") != nil {
		t.Fatal("Get must return nil for unknown")
	}
	got := List()
	if len(got) < 4 {
		t.Fatalf("List() should include at least 4 vendors, got %d", len(got))
	}
}

func TestResolveAudioMIME_FromHeader(t *testing.T) {
	if m := resolveAudioMIME(Request{AudioMIME: "audio/wav"}); m != "audio/wav" {
		t.Fatalf("expected audio/wav, got %s", m)
	}
}

func TestResolveAudioMIME_FromFilename(t *testing.T) {
	if m := resolveAudioMIME(Request{FileName: "clip.mp3"}); m != "audio/mpeg" {
		t.Fatalf("expected audio/mpeg, got %s", m)
	}
	if m := resolveAudioMIME(Request{FileName: "clip.flac"}); m != "audio/flac" {
		t.Fatalf("expected audio/flac, got %s", m)
	}
	if m := resolveAudioMIME(Request{FileName: "clip"}); m != "application/octet-stream" {
		t.Fatalf("expected octet-stream, got %s", m)
	}
}

func TestDeepgram_BasicTranscribe(t *testing.T) {
	body := `{"results":{"channels":[{"alternatives":[{"transcript":"hello world","confidence":0.97,"language":"en"}]}]},"metadata":{"duration":1.23}}`
	res := runTranscribe(t, &deepgramProvider{}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("model") != "nova-2" {
			t.Errorf("expected model=nova-2 default, got %q", r.URL.Query().Get("model"))
		}
		if r.Header.Get("Authorization") != "Token tok-x" {
			t.Errorf("auth header wrong: %s", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, body)
	}, Request{APIKey: "tok-x", Audio: []byte("rawaudio")})
	if res.Text != "hello world" {
		t.Fatalf("text mismatch: %q", res.Text)
	}
	if res.Language != "en" || res.DurationSec != 1.23 {
		t.Fatalf("metadata mismatch: %+v", res)
	}
}

func TestOpenAI_MultipartShape(t *testing.T) {
	res := runTranscribe(t, &openaiProvider{}, func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected multipart, got %s", ct)
		}
		// Parse to confirm shape.
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		if r.FormValue("model") != "whisper-1" {
			t.Errorf("expected model=whisper-1, got %q", r.FormValue("model"))
		}
		_, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("missing file field: %v", err)
		}
		_, _ = io.WriteString(w, `{"text":"halo dunia","language":"id","duration":0.5}`)
	}, Request{APIKey: "sk-x", Audio: []byte("rawaudio"), FileName: "clip.mp3"})
	if res.Text != "halo dunia" || res.Language != "id" {
		t.Fatalf("openai parse mismatch: %+v", res)
	}
}

func TestGemini_Base64Inline(t *testing.T) {
	res := runTranscribe(t, &geminiProvider{}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "key-x" {
			t.Errorf("expected key query, got %q", r.URL.Query().Get("key"))
		}
		var sniff map[string]any
		_ = json.NewDecoder(r.Body).Decode(&sniff)
		// minimum shape check
		contents, _ := sniff["contents"].([]any)
		if len(contents) == 0 {
			t.Fatal("empty contents")
		}
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"hola mundo"}]}}]}`)
	}, Request{APIKey: "key-x", Audio: []byte("audio"), Language: "es", FileName: "voice.mp3"})
	if res.Text != "hola mundo" {
		t.Fatalf("gemini text mismatch: %q", res.Text)
	}
}

func TestAssemblyAI_HappyPath(t *testing.T) {
	stage := 0
	res := runTranscribe(t, &assemblyAIProvider{}, func(w http.ResponseWriter, r *http.Request) {
		stage++
		switch {
		case strings.HasSuffix(r.URL.Path, "/upload"):
			_, _ = io.WriteString(w, `{"upload_url":"https://fake/u/1"}`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/transcript"):
			_, _ = io.WriteString(w, `{"id":"job-1"}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/transcript/job-1"):
			// First poll: processing. Second poll: completed.
			if stage <= 3 {
				_, _ = io.WriteString(w, `{"status":"processing"}`)
				return
			}
			_, _ = io.WriteString(w, `{"status":"completed","text":"final","language_code":"en","audio_duration":2.0}`)
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusBadRequest)
		}
	}, Request{APIKey: "k", Audio: []byte("audio")})
	if res.Text != "final" {
		t.Fatalf("expected 'final', got %q", res.Text)
	}
}

func TestUpstreamErrorPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	p := &deepgramProvider{}
	_, err := p.Transcribe(context.Background(), Request{APIKey: "x", BaseURL: srv.URL, Audio: []byte("a")})
	if err == nil {
		t.Fatal("expected error on upstream 500")
	}
}
