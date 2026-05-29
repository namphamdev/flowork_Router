// Package mesh — mesh_db.go: SQLite persistence untuk M3 schema.
//
// File DB: <DataDir>/mesh/mesh.sqlite. Pure Go (modernc.org/sqlite, CGO=0).
// Pattern parity dengan brain/sqlite_store.go.
//
// 3 tabel inti per ARCHITECTURE.md + amendment v1:
//
//   peer_packets    — append-only signed records (FQP-12 No-Deletion)
//   peer_registry   — per-peer trust state (karma populated post-M5 migration)
//   shadow_drawers  — zona karantina pre-promote (L6-L9)
//
// Append-only enforcement via trigger: DELETE row di peer_packets = ABORT.
// Soft-delete via filter_status='invalidated' atau 'expired' (I-4 TTL).
//
// Concurrency: WAL mode + busy_timeout(10000ms). Singleton via Shared().

// Sprint 3.5e §1.2 split — implementations moved to multi-file modules:
//   - mesh_db_packets.go  : peer_packets CRUD
//   - mesh_db_peers.go    : peer_registry CRUD + ErrPeerNotFound
//   - mesh_db_shadows.go  : shadow_drawers CRUD
//   - mesh_db_stats.go    : aggregate stats
//
// All methods stay on `*MeshDB` (defined here) so receivers co-locate.

package mesh

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	kpath "github.com/flowork/kernel/kernel/path"

	_ "modernc.org/sqlite"
)

// ErrAppendOnly raised saat caller coba DELETE row peer_packets.
var ErrAppendOnly = errors.New("mesh: append-only constraint (FQP-12) — use Invalidate or MarkExpired")

// MeshDB wrap mesh.sqlite connection. Thread-safe (sql.DB native concurrency).
type MeshDB struct {
	db   *sql.DB
	path string
	mu   sync.RWMutex
}

var (
	sharedMeshDB     *MeshDB
	sharedMeshDBOnce sync.Once
	sharedMeshDBErr  error
)

// SharedMeshDB return singleton instance. First call triggers OpenMeshDB +
// migrate. Subsequent calls reuse instance. Thread-safe.
func SharedMeshDB() (*MeshDB, error) {
	sharedMeshDBOnce.Do(func() {
		sharedMeshDB, sharedMeshDBErr = OpenMeshDB()
	})
	return sharedMeshDB, sharedMeshDBErr
}

// OpenMeshDB buka / create mesh.sqlite di data dir. Auto-migrate. Idempotent.
func OpenMeshDB() (*MeshDB, error) {
	dataDir, err := kpath.DataDir()
	if err != nil {
		return nil, fmt.Errorf("mesh.OpenMeshDB: data dir: %w", err)
	}
	dbDir := filepath.Join(dataDir, "mesh")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, fmt.Errorf("mesh.OpenMeshDB: mkdir: %w", err)
	}
	dbPath := filepath.Join(dbDir, "mesh.sqlite")

	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("mesh.OpenMeshDB: open %q: %w", dbPath, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("mesh.OpenMeshDB: ping: %w", err)
	}

	m := &MeshDB{db: db, path: dbPath}
	if err := m.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("mesh.OpenMeshDB: migrate: %w", err)
	}
	return m, nil
}

// Close release DB connection. Idempotent.
func (m *MeshDB) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db == nil {
		return nil
	}
	err := m.db.Close()
	m.db = nil
	return err
}

// Path return absolut path file.
func (m *MeshDB) Path() string { return m.path }

// migrate idempotent CREATE TABLE IF NOT EXISTS + index + trigger.
func (m *MeshDB) migrate() error {
	stmts := []string{
		// peer_packets — append-only signed records
		`CREATE TABLE IF NOT EXISTS peer_packets (
			id            TEXT PRIMARY KEY,
			type          TEXT NOT NULL,
			payload       BLOB NOT NULL,
			author_pubkey BLOB NOT NULL,
			license_id    TEXT NOT NULL,
			signature     BLOB NOT NULL,
			parent_id     TEXT,
			amplitude     REAL NOT NULL,
			timestamp     INTEGER NOT NULL,
			hop_count     INTEGER NOT NULL DEFAULT 0,
			filter_status TEXT NOT NULL,
			promoted_at   INTEGER,
			valid_to      INTEGER,
			expires_at    INTEGER,
			audit_log     TEXT,
			CHECK (filter_status IN ('shadow','quarantine','promoted','invalidated','expired','dropped'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_packet_status    ON peer_packets(filter_status)`,
		`CREATE INDEX IF NOT EXISTS idx_packet_author    ON peer_packets(author_pubkey)`,
		`CREATE INDEX IF NOT EXISTS idx_packet_timestamp ON peer_packets(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_packet_expires   ON peer_packets(expires_at) WHERE expires_at IS NOT NULL`,

		// peer_registry — per peer trust state
		`CREATE TABLE IF NOT EXISTS peer_registry (
			pubkey         BLOB PRIMARY KEY,
			license_id     TEXT NOT NULL,
			first_seen     INTEGER NOT NULL,
			last_seen      INTEGER NOT NULL,
			karma          REAL NOT NULL DEFAULT 0.5,
			packets_sent   INTEGER NOT NULL DEFAULT 0,
			packets_drop   INTEGER NOT NULL DEFAULT 0,
			packets_promo  INTEGER NOT NULL DEFAULT 0,
			endorsed_by    TEXT,
			banned_until   INTEGER,
			is_virtualized INTEGER NOT NULL DEFAULT 0,
			CHECK (karma >= 0.0 AND karma <= 1.0)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_peer_karma     ON peer_registry(karma)`,
		`CREATE INDEX IF NOT EXISTS idx_peer_last_seen ON peer_registry(last_seen)`,

		// shadow_drawers — zona karantina pre-promote
		`CREATE TABLE IF NOT EXISTS shadow_drawers (
			packet_id            TEXT PRIMARY KEY REFERENCES peer_packets(id),
			drawer_content       TEXT NOT NULL,
			cosine_score         REAL NOT NULL DEFAULT -1,
			consensus_count      INTEGER NOT NULL DEFAULT 1,
			shadow_since         INTEGER NOT NULL,
			scheduled_promote_at INTEGER NOT NULL,
			user_feedback        TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_shadow_promote  ON shadow_drawers(scheduled_promote_at)`,
		`CREATE INDEX IF NOT EXISTS idx_shadow_feedback ON shadow_drawers(user_feedback)`,

		// Append-only enforcement (FQP-12)
		`CREATE TRIGGER IF NOT EXISTS no_delete_packets
			BEFORE DELETE ON peer_packets
			BEGIN
				SELECT RAISE(ABORT, 'append-only: peer_packets row cannot be deleted (FQP-12)');
			END`,
	}

	for i, s := range stmts {
		if _, err := m.db.Exec(s); err != nil {
			return fmt.Errorf("migrate stmt %d: %w", i, err)
		}
	}
	return nil
}
