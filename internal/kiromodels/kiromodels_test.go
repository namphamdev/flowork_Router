// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — kiromodels (Kiro provider models registry).

package kiromodels

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExpandVariants_FourPerBase(t *testing.T) {
	got := expandVariants([]Model{{ID: "claude-sonnet-4"}, {ID: "claude-opus-4"}})
	if len(got) != 8 {
		t.Fatalf("expected 8 (4×2), got %d", len(got))
	}
	wantIDs := map[string]bool{
		"claude-sonnet-4": false, "claude-sonnet-4-thinking": false,
		"claude-sonnet-4-agentic": false, "claude-sonnet-4-thinking-agentic": false,
		"claude-opus-4": false, "claude-opus-4-thinking": false,
		"claude-opus-4-agentic": false, "claude-opus-4-thinking-agentic": false,
	}
	for _, m := range got {
		if _, ok := wantIDs[m.ID]; !ok {
			t.Errorf("unexpected id: %s", m.ID)
		}
		wantIDs[m.ID] = true
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Errorf("missing id: %s", id)
		}
	}
}

func TestExpandVariants_SyntheticFlag(t *testing.T) {
	got := expandVariants([]Model{{ID: "x"}})
	if got[0].Synthetic {
		t.Fatal("base must NOT be marked synthetic")
	}
	for i := 1; i < 4; i++ {
		if !got[i].Synthetic {
			t.Errorf("variant %s missing synthetic flag", got[i].ID)
		}
	}
}

func TestStripSyntheticSuffixes(t *testing.T) {
	cases := map[string]string{
		"claude-sonnet-4-thinking-agentic": "claude-sonnet-4",
		"claude-sonnet-4-agentic":          "claude-sonnet-4",
		"claude-sonnet-4-thinking":         "claude-sonnet-4",
		"claude-sonnet-4":                  "claude-sonnet-4",
		"plain":                            "plain",
	}
	for in, want := range cases {
		if got := stripSyntheticSuffixes(in); got != want {
			t.Errorf("strip(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRegionFromProfileArn(t *testing.T) {
	cases := map[string]string{
		"arn:aws:codewhisperer:us-east-1:123:profile/abc": "us-east-1",
		"arn:aws:codewhisperer:eu-west-2:999:profile/xyz": "eu-west-2",
		"":      defaultRegion,
		"bogus": defaultRegion,
	}
	for in, want := range cases {
		if got := regionFromProfileArn(in); got != want {
			t.Errorf("region(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFetch_HappyPathExpandsVariants(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("auth header wrong: %q", got)
		}
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "aws-sdk-js") {
			t.Errorf("UA missing aws-sdk-js: %q", ua)
		}
		_, _ = io.WriteString(w, `{"models":[
			{"modelId":"claude-sonnet-4","modelName":"Claude Sonnet 4","provider":"anthropic"},
			{"modelId":"amazon-nova","modelName":"Amazon Nova","provider":"amazon"}
		]}`)
	}))
	defer srv.Close()

	orig := httpClient
	defer func() { httpClient = orig }()
	httpClient = &http.Client{Transport: rewriteTransport{target: srv.URL}}

	InvalidateCache()
	cat, err := Fetch(context.Background(), Params{Token: "tok", Region: "us-east-1"})
	if err != nil {
		t.Fatal(err)
	}
	if cat.Region != "us-east-1" {
		t.Errorf("region: %s", cat.Region)
	}
	if len(cat.Models) != 8 {
		t.Fatalf("expected 8 models (2 base × 4 variants), got %d", len(cat.Models))
	}
}

func TestFetch_CacheHits(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.WriteString(w, `{"models":[{"modelId":"m1"}]}`)
	}))
	defer srv.Close()

	orig := httpClient
	defer func() { httpClient = orig }()
	httpClient = &http.Client{Transport: rewriteTransport{target: srv.URL}}

	InvalidateCache()
	for i := 0; i < 3; i++ {
		_, err := Fetch(context.Background(), Params{Token: "tok-cached", Region: "us-east-1"})
		if err != nil {
			t.Fatal(err)
		}
	}
	if hits != 1 {
		t.Fatalf("expected 1 upstream hit (cache), got %d", hits)
	}
}

func TestFetch_RequiresToken(t *testing.T) {
	_, err := Fetch(context.Background(), Params{})
	if err == nil {
		t.Fatal("expected error without token")
	}
}

// rewriteTransport rewrites scheme+host to point at a test server.
type rewriteTransport struct{ target string }

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if i := strings.Index(rt.target, "://"); i > 0 {
		req.URL.Scheme = rt.target[:i]
		req.URL.Host = rt.target[i+3:]
	}
	return http.DefaultTransport.RoundTrip(req)
}
