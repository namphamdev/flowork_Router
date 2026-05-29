// Package mesh — mesh_db_stats.go: aggregate stats untuk /v1/mesh/stats.
//
// Sprint 3.5e §1.2 split — moved from mesh_db.go.

package mesh

import (
	"fmt"
	"time"
)

// Stats return agregat untuk endpoint /v1/mesh/stats.
func (m *MeshDB) Stats() (MeshStats, error) {
	var s MeshStats

	// Packets by status
	// hunting_bug 2026-04-30 BUG-004 fix: pakai defer rows.Close() + check
	// rows.Err() — sebelumnya manual close + no rows.Err() check, early
	// termination (timeout/corrupt WAL) bisa undercount silently.
	rows, err := m.db.Query(`SELECT filter_status, COUNT(*) FROM peer_packets GROUP BY filter_status`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			rows.Close()
			return s, err
		}
		s.Packets.Total += n
		switch st {
		case FilterStatusShadow:
			s.Packets.Shadow = n
		case FilterStatusQuarantine:
			s.Packets.Quarantine = n
		case FilterStatusPromoted:
			s.Packets.Promoted = n
		case FilterStatusInvalidated:
			s.Packets.Invalidated = n
		case FilterStatusExpired:
			s.Packets.Expired = n
		case FilterStatusDropped:
			s.Packets.Dropped = n
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return s, fmt.Errorf("Stats: rows.Err packets: %w", err)
	}
	rows.Close()

	// Peers
	now := time.Now().Unix()
	cutoff := now - 24*3600
	err = m.db.QueryRow(`
		SELECT
		    COUNT(*),
		    COALESCE(SUM(CASE WHEN last_seen >= ? THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN banned_until IS NOT NULL AND banned_until > ? THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(is_virtualized), 0),
		    COALESCE(AVG(karma), 0)
		FROM peer_registry
	`, cutoff, now).Scan(&s.Peers.Total, &s.Peers.Active24h, &s.Peers.Banned,
		&s.Peers.Virtualized, &s.Peers.AvgKarma)
	if err != nil {
		return s, err
	}
	return s, nil
}
