// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file. Standard test patterns, no production race/leak risk.

package brain

import (
	"context"
	"testing"
)

// TestLiveRetrieve exercises the bridge against a REAL Memory Palace DB.
// It is a no-op unless FLOW_ROUTER_BRAIN_DB (or the default path) points at an
// existing DB, so it never fails in CI or on machines without the brain.
//
//	FLOW_ROUTER_BRAIN_DB=/path/to/flowork-brain.sqlite go test ./internal/brain/ -run Live -v
func TestLiveRetrieve(t *testing.T) {
	if !Available() {
		t.Skipf("no brain DB at %q — skipping live test", DBPath())
	}
	db, err := Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()

	for _, q := range []string{"sql injection", "xss vulnerability", "reverse shell"} {
		snips, err := Retrieve(ctx, db, q, RetrieveOpts{Limit: 3, MaxContentLen: 160})
		if err != nil {
			t.Fatalf("retrieve %q: %v", q, err)
		}
		t.Logf("query %q → %d snippets", q, len(snips))
		for i, s := range snips {
			t.Logf("  [%d] wing=%s score=%.3f drawer=%s :: %s", i, s.Wing, s.Score, s.DrawerID, s.Content)
		}
		if len(snips) == 0 {
			t.Errorf("expected snippets for %q", q)
		}
	}
}
