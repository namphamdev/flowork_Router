// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package mitm

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestCertManager_RootCreatedAndPersisted(t *testing.T) {
	tmp := t.TempDir()
	m, err := NewCertManager(tmp)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if m.RootCAPEM() == nil || len(m.RootCAPEM()) < 100 {
		t.Fatalf("rootCA PEM looks empty: %d bytes", len(m.RootCAPEM()))
	}
	// Disk artifacts must exist.
	if _, err := os.Stat(filepath.Join(tmp, "mitm", "rootCA.pem")); err != nil {
		t.Fatalf("rootCA.pem missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "mitm", "rootCA.key")); err != nil {
		t.Fatalf("rootCA.key missing: %v", err)
	}
	// Reopening must reuse the same root (not regen).
	pem1 := m.RootCAPEM()
	m2, err := NewCertManager(tmp)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if string(m2.RootCAPEM()) != string(pem1) {
		t.Fatal("rootCA regenerated on second open — should reuse from disk")
	}
}

func TestCertManager_LeafSignedByRoot(t *testing.T) {
	tmp := t.TempDir()
	m, err := NewCertManager(tmp)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	leaf, err := m.IssueLeaf("example.com")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if leaf.Certificate == nil || len(leaf.Certificate) == 0 {
		t.Fatal("leaf has no certificate bytes")
	}
	parsed, err := x509.ParseCertificate(leaf.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	// Verify chain: leaf must verify against our root.
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(m.RootCAPEM()) {
		t.Fatal("failed to add root to pool")
	}
	if _, err := parsed.Verify(x509.VerifyOptions{
		Roots:   pool,
		DNSName: "example.com",
	}); err != nil {
		t.Fatalf("leaf does not verify against root: %v", err)
	}
}

func TestCertManager_LeafCachedAcrossCalls(t *testing.T) {
	tmp := t.TempDir()
	m, err := NewCertManager(tmp)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	a, _ := m.IssueLeaf("api.cursor.sh")
	b, _ := m.IssueLeaf("api.cursor.sh")
	if a == nil || b == nil {
		t.Fatal("nil leaf")
	}
	// Pointer equality — second call should hit the in-memory cache.
	if a != b {
		t.Fatal("second call returned a different leaf pointer (cache miss)")
	}
}
