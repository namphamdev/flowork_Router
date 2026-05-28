// DB Migrations Framework.

package store

import (
	"database/sql"
	"fmt"
	"sort"
	"sync"
)

// Migration describes one ordered, idempotent schema change.
type Migration struct {
	ID   int    // monotonic, unique
	Name string // short slug for logs and the schemaMigrations table
	SQL  string // statements; runs inside a single transaction
}

var (
	migrationsMu sync.Mutex
	migrations   []Migration
)

// RegisterMigration adds a migration to the registry. Call from an init()
// function in a sibling file so the registry is populated before Open().
func RegisterMigration(m Migration) {
	migrationsMu.Lock()
	defer migrationsMu.Unlock()
	migrations = append(migrations, m)
}

// applyMigrations runs every registered migration with id > the highest id
// already recorded in schemaMigrations. Called from Open() after the base
// schema is in place. Safe to call repeatedly.
func applyMigrations(d *sql.DB) error {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schemaMigrations (
		id        INTEGER PRIMARY KEY,
		name      TEXT NOT NULL,
		appliedAt TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schemaMigrations: %w", err)
	}

	migrationsMu.Lock()
	pending := append([]Migration(nil), migrations...)
	migrationsMu.Unlock()
	if len(pending) == 0 {
		return nil
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].ID < pending[j].ID })

	// Filter out already-applied
	applied := map[int]bool{}
	rows, err := d.Query(`SELECT id FROM schemaMigrations`)
	if err != nil {
		return fmt.Errorf("query schemaMigrations: %w", err)
	}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			applied[id] = true
		}
	}
	rows.Close()

	var todo []Migration
	for _, m := range pending {
		if !applied[m.ID] {
			todo = append(todo, m)
		}
	}
	if len(todo) == 0 {
		return nil
	}

	// Best-effort pre-migration snapshot using the SAME conn we are migrating
	// against. MUST NOT call Backup() here — Backup() goes through Open() which
	// re-enters sync.Once and deadlocks (this function runs INSIDE dbOnce.Do).
	_, _ = backupWithConn(d, "pre-migrate", defaultKeepBackups)

	for _, m := range todo {
		tx, err := d.Begin()
		if err != nil {
			return fmt.Errorf("begin %d: %w", m.ID, err)
		}
		if _, err := tx.Exec(m.SQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d %q: %w", m.ID, m.Name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schemaMigrations (id, name) VALUES (?, ?)`, m.ID, m.Name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.ID, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %d: %w", m.ID, err)
		}
	}
	return nil
}

// MigrationStatus reports applied vs pending for the dashboard UI / debug.
type MigrationStatus struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Applied   bool   `json:"applied"`
	AppliedAt string `json:"appliedAt,omitempty"`
}

// ListMigrationStatus returns one record per registered migration, with the
// applied/timestamp filled when present in schemaMigrations.
func ListMigrationStatus(d *sql.DB) ([]MigrationStatus, error) {
	migrationsMu.Lock()
	all := append([]Migration(nil), migrations...)
	migrationsMu.Unlock()
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	applied := map[int]string{}
	rows, err := d.Query(`SELECT id, appliedAt FROM schemaMigrations`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int
		var at string
		if err := rows.Scan(&id, &at); err == nil {
			applied[id] = at
		}
	}
	rows.Close()

	out := make([]MigrationStatus, 0, len(all))
	for _, m := range all {
		s := MigrationStatus{ID: m.ID, Name: m.Name}
		if at, ok := applied[m.ID]; ok {
			s.Applied = true
			s.AppliedAt = at
		}
		out = append(out, s)
	}
	return out, nil
}
