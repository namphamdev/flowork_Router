// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCLI_StatusAgainstFakeRouter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"service":"flow_router","status":"ok","version":"test","uptime":42}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	old := rootURL
	t.Cleanup(func() { rootURL = old })
	rootURL = srv.URL

	// Capture stdout
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = stdout })

	if err := cmdStatus(); err != nil {
		t.Fatalf("cmdStatus: %v", err)
	}
	w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	for _, want := range []string{"service:", "flow_router", "version:", "test", "uptime:", "42s"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q: %s", want, out)
		}
	}
}

func TestCLI_ProvidersListShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"p1","name":"OpenAI","provider":"openai","isActive":true,"priority":1}]}`))
	}))
	defer srv.Close()

	old := rootURL
	t.Cleanup(func() { rootURL = old })
	rootURL = srv.URL

	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = stdout })

	if err := cmdProviders(); err != nil {
		t.Fatalf("providers: %v", err)
	}
	w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	for _, want := range []string{"id", "name", "active", "p1", "OpenAI"} {
		if !strings.Contains(out, want) {
			t.Fatalf("providers output missing %q: %s", want, out)
		}
	}
}

func TestCLI_KeysNewBodyShape(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"k1","name":"my-key"}`))
	}))
	defer srv.Close()
	old := rootURL
	t.Cleanup(func() { rootURL = old })
	rootURL = srv.URL

	if err := cmdKeys([]string{"new", "my-key"}); err != nil {
		t.Fatalf("keys new: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/api/keys" {
		t.Fatalf("expected POST /api/keys, got %s %s", gotMethod, gotPath)
	}
	if !strings.Contains(gotBody, `"name":"my-key"`) {
		t.Fatalf("body missing name: %s", gotBody)
	}
}
