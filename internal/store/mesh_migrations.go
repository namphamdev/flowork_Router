// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flow_router
// Locked at: 2026-05-30
// Reason: Section 13 phase 1 migration. ID 100 reserved untuk mesh.
//   Phase 2+ migration (peer_groups, gossip_state, mesh_packets) → ID
//   100..199 reserved range. JANGAN re-use ID 100.
//
// mesh_migrations.go — Section 13 phase 1 schema: mesh_identity + mesh_peers.

package store

func init() {
	RegisterMigration(Migration{
		ID:   100,
		Name: "section13_mesh_foundation",
		SQL: `
CREATE TABLE IF NOT EXISTS mesh_identity (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL DEFAULT ''
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS mesh_peers (
  pubkey_hex     TEXT PRIMARY KEY,
  hostname       TEXT NOT NULL DEFAULT '',
  ip             TEXT NOT NULL DEFAULT '',
  port           INTEGER NOT NULL DEFAULT 0,
  version        TEXT NOT NULL DEFAULT '',
  is_virt        INTEGER NOT NULL DEFAULT 0,
  first_seen_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  trust_score    REAL NOT NULL DEFAULT 0.5,
  blocked        INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_mesh_peers_lastseen ON mesh_peers(last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_mesh_peers_blocked ON mesh_peers(blocked);
`,
	})
}
