// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackup_VacuumIntoOrCopyCreatesValidSnapshot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("FLOW_ROUTER_DATA", tmp)
	resetDBSingletonForTest()

	d, err := Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Seed a row so we can verify backup contents diverge from an empty file.
	if _, err := d.Exec(`INSERT INTO kv (k, v, updatedAt) VALUES ('backup-test', 'ok', datetime('now'))`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	info, err := Backup("unit", 3)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if info.SizeBytes <= 0 {
		t.Fatalf("snapshot is empty: %+v", info)
	}
	if !strings.HasPrefix(info.Label, "unit") {
		t.Fatalf("unexpected label %q", info.Label)
	}
	if _, err := os.Stat(info.DBPath); err != nil {
		t.Fatalf("snapshot db missing: %v", err)
	}
	if got := filepath.Dir(info.DBPath); got != info.Dir {
		t.Fatalf("dir mismatch: %s vs %s", got, info.Dir)
	}
}

func TestBackup_PrunesOldestBeyondKeepN(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("FLOW_ROUTER_DATA", tmp)
	resetDBSingletonForTest()

	if _, err := Open(); err != nil {
		t.Fatalf("open: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := Backup("rolling", 3); err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
	}
	list, err := ListBackups()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) > 3 {
		t.Fatalf("expected ≤3 snapshots after prune, got %d", len(list))
	}
}
