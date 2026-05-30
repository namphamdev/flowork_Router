// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package cloudcode

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// rewriteDoer routes ALL outbound CloudCode URLs to the test server.
type rewriteDoer struct{ target string }

func (rd rewriteDoer) Do(req *http.Request) (*http.Response, error) {
	if i := strings.Index(rd.target, "://"); i > 0 {
		req.URL.Scheme = rd.target[:i]
		req.URL.Host = rd.target[i+3:]
	}
	return http.DefaultTransport.RoundTrip(req)
}

func setupServer(t *testing.T, handler http.HandlerFunc) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	prev := httpDoer
	SetHTTPDoer(rewriteDoer{target: srv.URL})
	InvalidateAll()
	return srv.URL, func() {
		srv.Close()
		SetHTTPDoer(prev)
		InvalidateAll()
	}
}

func TestExtractProjectID_FromCloudAICompanionField(t *testing.T) {
	m := map[string]any{"cloudaicompanionProject": "my-project-123"}
	if got := extractProjectID(m); got != "my-project-123" {
		t.Fatalf("expected my-project-123, got %s", got)
	}
}

func TestExtractProjectID_FromNestedObject(t *testing.T) {
	m := map[string]any{
		"cloudaicompanionProject": map[string]any{"id": "nested-id"},
	}
	if got := extractProjectID(m); got != "nested-id" {
		t.Fatalf("expected nested-id, got %s", got)
	}
}

func TestExtractProjectID_FromFallbackField(t *testing.T) {
	m := map[string]any{"project": "fallback-project"}
	if got := extractProjectID(m); got != "fallback-project" {
		t.Fatalf("expected fallback-project, got %s", got)
	}
}

func TestExtractProjectID_EmptyOnMissing(t *testing.T) {
	if got := extractProjectID(map[string]any{}); got != "" {
		t.Fatalf("expected empty, got %s", got)
	}
}

func TestGetProjectID_HappyPath(t *testing.T) {
	hits := int32(0)
	_, cleanup := setupServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("Authorization") != "Bearer tok-1" {
			t.Errorf("auth header wrong: %s", r.Header.Get("Authorization"))
		}
		if !strings.Contains(r.Header.Get("Client-Metadata"), "ANTIGRAVITY") {
			t.Errorf("client-metadata missing ANTIGRAVITY: %s", r.Header.Get("Client-Metadata"))
		}
		_, _ = io.WriteString(w, `{"cloudaicompanionProject":"real-project-xyz"}`)
	})
	defer cleanup()

	pid, err := GetProjectID(context.Background(), "conn-1", "tok-1")
	if err != nil {
		t.Fatal(err)
	}
	if pid != "real-project-xyz" {
		t.Fatalf("got %s", pid)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", hits)
	}
}

func TestGetProjectID_CachesAcrossCalls(t *testing.T) {
	hits := int32(0)
	_, cleanup := setupServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = io.WriteString(w, `{"cloudaicompanionProject":"cached-p"}`)
	})
	defer cleanup()

	for i := 0; i < 3; i++ {
		_, err := GetProjectID(context.Background(), "conn-cache", "tok-x")
		if err != nil {
			t.Fatal(err)
		}
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected exactly 1 upstream hit (cache), got %d", hits)
	}
}

func TestGetProjectID_DeduplicatesConcurrentFetch(t *testing.T) {
	hits := int32(0)
	_, cleanup := setupServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		// Simulate slow upstream so concurrent callers wait on the same in-flight fetch.
		_, _ = io.WriteString(w, `{"cloudaicompanionProject":"deduped"}`)
	})
	defer cleanup()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = GetProjectID(context.Background(), "conn-conc", "tok-x")
		}()
	}
	wg.Wait()
	if h := atomic.LoadInt32(&hits); h > 2 {
		t.Fatalf("expected ≤2 hits via dedup, got %d", h)
	}
}

func TestGetProjectID_RequiresArgs(t *testing.T) {
	if _, err := GetProjectID(context.Background(), "", "tok"); err == nil {
		t.Fatal("empty connectionID should error")
	}
	if _, err := GetProjectID(context.Background(), "c", ""); err == nil {
		t.Fatal("empty token should error")
	}
}

func TestGetProjectID_OnboardFallbackWhenLoadEmpty(t *testing.T) {
	loadHits := int32(0)
	onboardHits := int32(0)
	_, cleanup := setupServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "onboardUser") {
			atomic.AddInt32(&onboardHits, 1)
			_, _ = io.WriteString(w, `{"cloudaicompanionProject":"onboarded-p"}`)
			return
		}
		atomic.AddInt32(&loadHits, 1)
		// loadCodeAssist returns no project but advertises a default tier.
		_, _ = io.WriteString(w, `{"allowedTiers":[{"id":"standard-tier","isDefault":true}]}`)
	})
	defer cleanup()

	pid, err := GetProjectID(context.Background(), "conn-onb", "tok-onb")
	if err != nil {
		t.Fatal(err)
	}
	if pid != "onboarded-p" {
		t.Fatalf("expected onboarded-p, got %s", pid)
	}
	if atomic.LoadInt32(&loadHits) != 1 || atomic.LoadInt32(&onboardHits) != 1 {
		t.Fatalf("expected 1+1, got load=%d onb=%d", loadHits, onboardHits)
	}
}

func TestGetProjectID_UpstreamErrorPropagates(t *testing.T) {
	_, cleanup := setupServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := GetProjectID(context.Background(), "conn-err", "tok-err")
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestInvalidate_ForcesRefetch(t *testing.T) {
	hits := int32(0)
	_, cleanup := setupServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = io.WriteString(w, `{"cloudaicompanionProject":"id-1"}`)
	})
	defer cleanup()

	_, _ = GetProjectID(context.Background(), "conn-inv", "tok")
	_, _ = GetProjectID(context.Background(), "conn-inv", "tok")
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("cache should suppress 2nd call, got %d", hits)
	}
	Invalidate("conn-inv")
	_, _ = GetProjectID(context.Background(), "conn-inv", "tok")
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("after Invalidate, should refetch (2), got %d", hits)
	}
}

func TestPlatformEnum_KnownGOOS(t *testing.T) {
	got := platformEnum()
	switch got {
	case "DARWIN_AMD64", "WINDOWS_AMD64", "LINUX_AMD64":
		// ok
	default:
		t.Fatalf("unexpected platform enum: %s", got)
	}
}
