// Package mesh — mesh_db_peers.go: peer_registry CRUD operations.
//
// Sprint 3.5e §1.2 split — moved from mesh_db.go.

package mesh

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrPeerNotFound returned by GetPeer kalau pubkey ngga ada di registry.
var ErrPeerNotFound = errors.New("mesh: peer not found")

// UpsertPeer insert or update peer entry. last_seen always update; karma +
// counters preserved. Untuk first-seen, set first_seen=now. is_virtualized
// hanya di-set saat first insert.
func (m *MeshDB) UpsertPeer(p PeerInfo) error {
	now := time.Now().Unix()
	first := p.FirstSeen.Unix()
	if first == 0 {
		first = now
	}
	last := p.LastSeen.Unix()
	if last == 0 {
		last = now
	}
	karma := p.Karma
	if karma == 0 {
		karma = 0.5
		if p.IsVirtualized {
			karma = 0.3
		}
	}
	virt := 0
	if p.IsVirtualized {
		virt = 1
	}
	endorsed := strings.Join(p.EndorsedBy, ",")

	_, err := m.db.Exec(`
		INSERT INTO peer_registry
		(pubkey, license_id, first_seen, last_seen, karma, is_virtualized, endorsed_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(pubkey) DO UPDATE SET
		    last_seen   = excluded.last_seen,
		    license_id  = excluded.license_id
	`, p.PubKey, p.LicenseID, first, last, karma, virt, endorsed)
	return err
}

// GetPeer fetch peer by pubkey. Return ErrPeerNotFound kalau ngga ada.
func (m *MeshDB) GetPeer(pubkey []byte) (*PeerInfo, error) {
	var p PeerInfo
	var endorsed sql.NullString
	var bannedUntil sql.NullInt64
	var virt int
	var first, last int64

	err := m.db.QueryRow(`
		SELECT pubkey, license_id, first_seen, last_seen, karma,
		       packets_sent, packets_drop, packets_promo,
		       endorsed_by, banned_until, is_virtualized
		FROM peer_registry WHERE pubkey = ?
	`, pubkey).Scan(&p.PubKey, &p.LicenseID, &first, &last, &p.Karma,
		&p.PacketsSent, &p.PacketsDrop, &p.PacketsPromo,
		&endorsed, &bannedUntil, &virt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("mesh: peer %s: %w", hex.EncodeToString(pubkey)[:8], ErrPeerNotFound)
		}
		return nil, err
	}
	p.FirstSeen = time.Unix(first, 0).UTC()
	p.LastSeen = time.Unix(last, 0).UTC()
	p.IsVirtualized = virt == 1
	if endorsed.Valid && endorsed.String != "" {
		p.EndorsedBy = strings.Split(endorsed.String, ",")
	}
	if bannedUntil.Valid {
		t := time.Unix(bannedUntil.Int64, 0).UTC()
		p.BannedUntil = &t
	}
	return &p, nil
}

// IncrementPeerCounter increment counter atomik (sent/drop/promo). col harus
// salah satu dari "packets_sent", "packets_drop", "packets_promo" — caller
// kasih literal const, bukan user input (anti SQL injection).
func (m *MeshDB) IncrementPeerCounter(pubkey []byte, col string) error {
	switch col {
	case "packets_sent", "packets_drop", "packets_promo":
		// ok
	default:
		return fmt.Errorf("invalid counter column: %q", col)
	}
	q := fmt.Sprintf(`UPDATE peer_registry SET %s = %s + 1, last_seen = ? WHERE pubkey = ?`, col, col)
	_, err := m.db.Exec(q, time.Now().Unix(), pubkey)
	return err
}

// ListPeers return semua peer di registry. Limit 1000 (anti memory blow).
func (m *MeshDB) ListPeers(limit int) ([]PeerInfo, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := m.db.Query(`
		SELECT pubkey, license_id, first_seen, last_seen, karma,
		       packets_sent, packets_drop, packets_promo,
		       endorsed_by, banned_until, is_virtualized
		FROM peer_registry
		ORDER BY last_seen DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PeerInfo
	for rows.Next() {
		var p PeerInfo
		var endorsed sql.NullString
		var bannedUntil sql.NullInt64
		var virt int
		var first, last int64
		if err := rows.Scan(&p.PubKey, &p.LicenseID, &first, &last, &p.Karma,
			&p.PacketsSent, &p.PacketsDrop, &p.PacketsPromo,
			&endorsed, &bannedUntil, &virt); err != nil {
			return nil, err
		}
		p.FirstSeen = time.Unix(first, 0).UTC()
		p.LastSeen = time.Unix(last, 0).UTC()
		p.IsVirtualized = virt == 1
		if endorsed.Valid && endorsed.String != "" {
			p.EndorsedBy = strings.Split(endorsed.String, ",")
		}
		if bannedUntil.Valid {
			t := time.Unix(bannedUntil.Int64, 0).UTC()
			p.BannedUntil = &t
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
