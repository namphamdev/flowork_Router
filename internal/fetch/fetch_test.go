package fetch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistry_RoundTrip(t *testing.T) {
	for _, name := range []string{"raw", "jina", "firecrawl"} {
		if Get(name) == nil {
			t.Fatalf("provider %q not registered", name)
		}
	}
	if Get("nonexistent") != nil {
		t.Fatal("Get must return nil for unknown")
	}
	if len(List()) < 3 {
		t.Fatalf("List() should include ≥ 3 vendors, got %d", len(List()))
	}
}

func TestRaw_FetchesUpstreamAsIs(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<html><body>hi</body></html>")
	}))
	defer upstream.Close()

	res, err := (&rawProvider{}).Fetch(context.Background(), Request{URL: upstream.URL})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Body), "<body>hi</body>") {
		t.Fatalf("body mismatch: %s", res.Body)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", res.StatusCode)
	}
	if !strings.HasPrefix(res.ContentType, "text/html") {
		t.Fatalf("content-type: %s", res.ContentType)
	}
}

func TestRaw_EmptyURLError(t *testing.T) {
	_, err := (&rawProvider{}).Fetch(context.Background(), Request{})
	if err == nil {
		t.Fatal("expected error on empty URL")
	}
}

func TestJina_PrefixesBaseToTarget(t *testing.T) {
	var got string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Path
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = io.WriteString(w, "# title\n\nbody")
	}))
	defer upstream.Close()

	res, err := (&jinaProvider{}).Fetch(context.Background(), Request{
		URL:     "https://example.com/page",
		BaseURL: upstream.URL,
		APIKey:  "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "/https://example.com/page") {
		t.Fatalf("expected target url appended, got %q", got)
	}
	if string(res.Body) != "# title\n\nbody" {
		t.Fatalf("body: %q", res.Body)
	}
}

func TestFirecrawl_HappyPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fc-key" {
			t.Errorf("auth: %s", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, `{"success":true,"data":{"markdown":"# Hi","metadata":{"title":"Test"}}}`)
	}))
	defer upstream.Close()

	res, err := (&firecrawlProvider{}).Fetch(context.Background(), Request{
		URL:     "https://example.com",
		APIKey:  "fc-key",
		BaseURL: upstream.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "Test" || string(res.Body) != "# Hi" {
		t.Fatalf("parsed wrong: %+v body=%s", res, res.Body)
	}
}

func TestFirecrawl_RequiresAPIKey(t *testing.T) {
	_, err := (&firecrawlProvider{}).Fetch(context.Background(), Request{URL: "https://example.com"})
	if err == nil {
		t.Fatal("expected error without api key")
	}
}
