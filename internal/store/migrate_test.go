package store

import "testing"

func TestMigrations_RunOnceThenIdempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("FLOW_ROUTER_DATA", tmp)
	resetDBSingletonForTest()

	// Register a one-off test migration. Use a high ID so it doesn't collide
	// with anything real registered in production code.
	RegisterMigration(Migration{
		ID:   99001,
		Name: "test-add-marker-table",
		SQL:  `CREATE TABLE IF NOT EXISTS migrate_marker (k TEXT PRIMARY KEY, v TEXT)`,
	})
	// Need to clear the slice so the registration above survives across tests
	// without re-adding on each test run; defer trim back.
	t.Cleanup(func() {
		migrationsMu.Lock()
		out := migrations[:0]
		for _, m := range migrations {
			if m.ID != 99001 {
				out = append(out, m)
			}
		}
		migrations = out
		migrationsMu.Unlock()
	})

	d, err := Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := d.Exec(`INSERT INTO migrate_marker (k, v) VALUES ('a', 'b')`); err != nil {
		t.Fatalf("marker should exist after first open: %v", err)
	}

	// Second Open() must be idempotent: same row remains, no error.
	resetDBSingletonForTest()
	d2, err := Open()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	var v string
	if err := d2.QueryRow(`SELECT v FROM migrate_marker WHERE k='a'`).Scan(&v); err != nil {
		t.Fatalf("row missing after reopen: %v", err)
	}
	if v != "b" {
		t.Fatalf("unexpected value %q", v)
	}

	// Status reports applied=true.
	statuses, err := ListMigrationStatus(d2)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	var found bool
	for _, s := range statuses {
		if s.ID == 99001 {
			found = true
			if !s.Applied {
				t.Fatal("status should report applied=true")
			}
		}
	}
	if !found {
		t.Fatal("test migration not present in status list")
	}
}
