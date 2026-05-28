package quotalive

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistry(t *testing.T) {
	for _, name := range []string{"claude", "copilot"} {
		if Get(name) == nil {
			t.Fatalf("fetcher %q not registered", name)
		}
	}
	if Get("nonexistent") != nil {
		t.Fatal("Get must return nil for unknown")
	}
	if len(List()) < 2 {
		t.Fatalf("List() should include ≥ 2 vendors, got %d", len(List()))
	}
}

func TestClaude_ParsesUtilizationWindows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-x" {
			t.Errorf("auth: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("anthropic-beta") == "" {
			t.Error("missing anthropic-beta header")
		}
		_, _ = io.WriteString(w, `{
			"plan":"pro",
			"five_hour":{"utilization":42,"resets_at":"2026-05-28T16:00:00Z"},
			"seven_day":{"utilization":17},
			"seven_day_sonnet":{"utilization":30}
		}`)
	}))
	defer srv.Close()

	// Override the URL by swapping httpClient — keep test pure by pointing the
	// transport at our local server via a one-off request reroute.
	// Simpler: patch the constant via a vendored test endpoint indirection.
	// Here we test the JSON parsing logic by faking the upstream URL through
	// http.DefaultTransport.RegisterProtocol — too heavy. Instead, exercise the
	// Snapshot parser by calling Fetch against the test server using a direct
	// dial via patching: we re-implement here by manual request, then call the
	// internal parser. But to keep this test honest we'll redirect via the
	// upstream's hostname using a custom round-trip.

	// Simpler again: just call Fetch with an injected URL via reflection isn't
	// available — but we can override the global httpClient transport to a
	// dialer that always hits srv. Easiest is to test by calling Fetch with a
	// rewritten httpClient.
	orig := httpClient
	defer func() { httpClient = orig }()
	httpClient = &http.Client{Transport: rewriteTransport{target: srv.URL}}

	snap, err := (&claudeFetcher{}).Fetch(context.Background(), Params{Token: "tok-x"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Plan != "pro" {
		t.Fatalf("plan: %s", snap.Plan)
	}
	if len(snap.Windows) < 3 {
		t.Fatalf("expected 3 windows, got %d: %+v", len(snap.Windows), snap.Windows)
	}
	// Check that "session (5h)" has Used=42 and Remaining=58
	found := false
	for _, w := range snap.Windows {
		if w.Label == "session (5h)" {
			if w.Used != 42 || w.Remaining != 58 || w.Unit != "percent" {
				t.Fatalf("session 5h wrong: %+v", w)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("session (5h) window not parsed")
	}
}

func TestCopilot_ParsesQuotaSnapshots(t *testing.T) {
	body := `{
		"copilot_plan":"business",
		"quota_reset_date":"2026-06-01",
		"quota_snapshots":{
			"chat":{"entitlement":300,"entitlement_used":47},
			"completions":{"entitlement":2000,"entitlement_used":1500,"unlimited":false}
		}
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token gh-pat" {
			t.Errorf("auth: %s", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	orig := httpClient
	defer func() { httpClient = orig }()
	httpClient = &http.Client{Transport: rewriteTransport{target: srv.URL}}

	snap, err := (&copilotFetcher{}).Fetch(context.Background(), Params{Token: "gh-pat"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Plan != "business" {
		t.Fatalf("plan: %s", snap.Plan)
	}
	if len(snap.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(snap.Windows))
	}
	for _, w := range snap.Windows {
		if w.Unit != "requests" {
			t.Errorf("unit: %s", w.Unit)
		}
		if w.Total <= 0 {
			t.Errorf("total: %f", w.Total)
		}
	}
}

func TestClaude_MissingToken(t *testing.T) {
	_, err := (&claudeFetcher{}).Fetch(context.Background(), Params{})
	if err == nil || !strings.Contains(err.Error(), "token required") {
		t.Fatalf("expected token-required error, got %v", err)
	}
}

func TestCopilot_MissingToken(t *testing.T) {
	_, err := (&copilotFetcher{}).Fetch(context.Background(), Params{})
	if err == nil || !strings.Contains(err.Error(), "token required") {
		t.Fatalf("expected token-required error, got %v", err)
	}
}

// rewriteTransport rewrites the outbound request's host:scheme to point at a
// local httptest server while keeping the original path/query/headers — lets
// us exercise Fetch without monkey-patching constants.
type rewriteTransport struct{ target string }

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL
	// Parse target → take scheme + host, keep original path/query.
	if i := strings.Index(rt.target, "://"); i > 0 {
		u.Scheme = rt.target[:i]
		u.Host = rt.target[i+3:]
	}
	return http.DefaultTransport.RoundTrip(req)
}
