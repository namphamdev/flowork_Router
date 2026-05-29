// Package mesh — mesh_db_packets.go: peer_packets CRUD operations.
//
// Sprint 3.5e §1.2 split — moved from mesh_db.go (anchor preserves migrate +
// types). All methods stay on *MeshDB so receivers co-locate.

package mesh

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// InsertPacket insert new KnowledgePacket dengan filter_status awal. Caller
// (filter pipeline M4) WAJIB pass status valid (lihat IsValidStatus).
//
// Idempotent via PRIMARY KEY id — duplicate insert error (caller harus check
// PacketExists dulu untuk loop prevention M6).
func (m *MeshDB) InsertPacket(p KnowledgePacket, status string) error {
	if !IsValidStatus(status) {
		return fmt.Errorf("invalid filter_status: %q", status)
	}
	if !IsValidPacketType(p.Type) {
		return fmt.Errorf("invalid packet type: %q", p.Type)
	}

	var expiresAt *int64
	if p.ExpiresAt != nil {
		v := p.ExpiresAt.Unix()
		expiresAt = &v
	}
	var parentID *string
	if p.ParentID != "" {
		parentID = &p.ParentID
	}

	_, err := m.db.Exec(`
		INSERT INTO peer_packets
		(id, type, payload, author_pubkey, license_id, signature, parent_id,
		 amplitude, timestamp, hop_count, filter_status, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID.String(), p.Type, p.Payload, p.AuthorPubKey, p.LicenseID,
		p.Signature, parentID, p.Amplitude, p.Timestamp.Unix(), p.HopCount,
		status, expiresAt)
	return err
}

// PacketExists return true kalau row dengan id ada (dedup check M6).
func (m *MeshDB) PacketExists(id string) (bool, error) {
	var n int
	err := m.db.QueryRow(`SELECT COUNT(1) FROM peer_packets WHERE id = ?`, id).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpdatePacketStatus transition status + append audit log line.
//
// Append-only: kalau row sudah `invalidated` atau `expired`, return error
// (terminal states, ngga boleh ke-flip lagi).
func (m *MeshDB) UpdatePacketStatus(id, newStatus, auditLine string) error {
	if !IsValidStatus(newStatus) {
		return fmt.Errorf("invalid filter_status: %q", newStatus)
	}
	res, err := m.db.Exec(`
		UPDATE peer_packets
		SET filter_status = ?,
		    audit_log     = COALESCE(audit_log, '') || ? ,
		    promoted_at   = CASE WHEN ? = 'promoted' AND promoted_at IS NULL THEN ? ELSE promoted_at END
		WHERE id = ?
		  AND filter_status NOT IN ('invalidated', 'expired')
	`, newStatus, "\n"+auditLine, newStatus, time.Now().Unix(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("packet %s not found or terminal status", id)
	}
	return nil
}

// InvalidatePacket set valid_to=now + status='invalidated'. Append-only
// (row tetap ada untuk audit trail).
func (m *MeshDB) InvalidatePacket(id, reason string) error {
	now := time.Now().Unix()
	res, err := m.db.Exec(`
		UPDATE peer_packets
		SET filter_status = 'invalidated',
		    valid_to      = ?,
		    audit_log     = COALESCE(audit_log, '') || ?
		WHERE id = ?
		  AND filter_status NOT IN ('invalidated', 'expired')
	`, now, "\ninvalidated: "+reason, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("packet %s not found or terminal status", id)
	}
	return nil
}

// MarkExpired sweep packet yang expires_at < now → status='expired'. Append-only.
// Return jumlah row affected. Dipanggil cron weekly (M3 amendment I-4).
func (m *MeshDB) MarkExpired() (int, error) {
	now := time.Now().Unix()
	res, err := m.db.Exec(`
		UPDATE peer_packets
		SET filter_status = 'expired',
		    audit_log     = COALESCE(audit_log, '') || ?
		WHERE expires_at IS NOT NULL
		  AND expires_at < ?
		  AND filter_status NOT IN ('expired', 'invalidated')
	`, "\nexpired by sweep at "+time.Now().UTC().Format(time.RFC3339), now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// GetPacket fetch single packet by id.
func (m *MeshDB) GetPacket(id string) (*KnowledgePacket, string, error) {
	var p KnowledgePacket
	var parentID sql.NullString
	var expiresAt sql.NullInt64
	var auditLog sql.NullString
	var ts int64
	var idStr string
	var status string

	err := m.db.QueryRow(`
		SELECT id, type, payload, author_pubkey, license_id, signature, parent_id,
		       amplitude, timestamp, hop_count, filter_status, expires_at, audit_log
		FROM peer_packets WHERE id = ?
	`, id).Scan(&idStr, &p.Type, &p.Payload, &p.AuthorPubKey, &p.LicenseID,
		&p.Signature, &parentID, &p.Amplitude, &ts, &p.HopCount, &status,
		&expiresAt, &auditLog)
	if err != nil {
		return nil, "", err
	}

	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		return nil, "", fmt.Errorf("packet id parse: %w", err)
	}
	p.ID = parsedID
	p.Timestamp = time.Unix(ts, 0).UTC()
	if parentID.Valid {
		p.ParentID = parentID.String
	}
	if expiresAt.Valid {
		t := time.Unix(expiresAt.Int64, 0).UTC()
		p.ExpiresAt = &t
	}
	auditStr := ""
	if auditLog.Valid {
		auditStr = auditLog.String
	}
	_ = auditStr // returned via separate accessor if needed
	return &p, status, nil
}

// ListPromotedSince — pull source untuk gossip M6. Return packet dengan
// status='promoted' dan timestamp > since. Cap limit.
func (m *MeshDB) ListPromotedSince(since int64, limit int) ([]KnowledgePacket, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := m.db.Query(`
		SELECT id, type, payload, author_pubkey, license_id, signature, parent_id,
		       amplitude, timestamp, hop_count
		FROM peer_packets
		WHERE filter_status = 'promoted'
		  AND timestamp > ?
		ORDER BY timestamp ASC
		LIMIT ?
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KnowledgePacket
	for rows.Next() {
		var p KnowledgePacket
		var parentID sql.NullString
		var ts int64
		var idStr string
		if err := rows.Scan(&idStr, &p.Type, &p.Payload, &p.AuthorPubKey,
			&p.LicenseID, &p.Signature, &parentID, &p.Amplitude, &ts,
			&p.HopCount); err != nil {
			return nil, err
		}
		p.ID, _ = uuid.Parse(idStr)
		p.Timestamp = time.Unix(ts, 0).UTC()
		if parentID.Valid {
			p.ParentID = parentID.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
