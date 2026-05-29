// Package mesh — mesh_db_shadows.go: shadow_drawers CRUD operations.
//
// Sprint 3.5e §1.2 split — moved from mesh_db.go.

package mesh

import (
	"fmt"
	"time"
)

// InsertShadow tambah row shadow zone untuk packet yang lulus L1-L5 + L6
// (filter pipeline M4). cosine_score awal -1 (belum di-validate L7).
func (m *MeshDB) InsertShadow(packetID, content string, scheduledPromoteAfter time.Duration) error {
	now := time.Now().Unix()
	scheduledAt := now + int64(scheduledPromoteAfter.Seconds())
	_, err := m.db.Exec(`
		INSERT INTO shadow_drawers
		(packet_id, drawer_content, cosine_score, consensus_count, shadow_since, scheduled_promote_at)
		VALUES (?, ?, -1, 1, ?, ?)
	`, packetID, content, now, scheduledAt)
	return err
}

// UpdateShadowCosine set cosine_score post-L7 validation.
func (m *MeshDB) UpdateShadowCosine(packetID string, cosine float64) error {
	_, err := m.db.Exec(`UPDATE shadow_drawers SET cosine_score = ? WHERE packet_id = ?`,
		cosine, packetID)
	return err
}

// IncrementShadowConsensus naikin consensus_count saat peer lain submit fakta
// serupa (L8 multi-source consensus check).
func (m *MeshDB) IncrementShadowConsensus(packetID string) error {
	_, err := m.db.Exec(`UPDATE shadow_drawers SET consensus_count = consensus_count + 1 WHERE packet_id = ?`,
		packetID)
	return err
}

// SetShadowFeedback set user_feedback untuk quarantine zone (I-1).
func (m *MeshDB) SetShadowFeedback(packetID, feedback string) error {
	switch feedback {
	case "thumbs_up", "thumbs_down", "":
		// ok
	default:
		return fmt.Errorf("invalid feedback: %q", feedback)
	}
	_, err := m.db.Exec(`UPDATE shadow_drawers SET user_feedback = ? WHERE packet_id = ?`,
		feedback, packetID)
	return err
}

// GetShadow fetch shadow entry by packet_id.
func (m *MeshDB) GetShadow(packetID string) (*ShadowEntry, error) {
	var e ShadowEntry
	var since, scheduled int64
	err := m.db.QueryRow(`
		SELECT packet_id, drawer_content, cosine_score, consensus_count,
		       shadow_since, scheduled_promote_at, user_feedback
		FROM shadow_drawers WHERE packet_id = ?
	`, packetID).Scan(&e.PacketID, &e.DrawerContent, &e.CosineScore,
		&e.ConsensusCount, &since, &scheduled, &e.UserFeedback)
	if err != nil {
		return nil, err
	}
	e.ShadowSince = time.Unix(since, 0).UTC()
	e.ScheduledPromoteAt = time.Unix(scheduled, 0).UTC()
	return &e, nil
}
