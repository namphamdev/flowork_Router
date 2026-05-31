// pipeline.go — Section 17+19+20 phase 3: end-to-end knowledge intake.
//
// This is the "glue" the audit found missing: incoming knowledge packets used
// to be persisted raw, with the 9-layer filter only reachable via the manual
// /api/mesh/filter/test endpoint. ProcessKnowledgePacket wires the real flow:
//
//	receive → 9-layer filter → near-dup check → karma update → inbox status
//
// so a hostile peer can no longer poison the local brain just by pushing a
// packet. Roadmap Section 20 marks this "wajib sebelum mesh public".

package mesh

import (
	"database/sql"
	"encoding/json"
	"strings"
)

// KnowledgeResult — outcome of processing one knowledge packet.
type KnowledgeResult struct {
	PacketID  string           `json:"packet_id"`
	Status    string           `json:"status"`    // shadow|quarantine|promoted|dropped
	Reason    string           `json:"reason"`    // human-readable summary
	Decisions []FilterDecision `json:"decisions"` // full 9-layer audit trail
	Duplicate bool             `json:"duplicate"`
}

// extractDrawerContent pulls the drawer text from a knowledge packet payload.
// Accepts {"drawer_content":"..."} (canonical) or {"content":"..."}; falls back
// to the raw payload string when it isn't a JSON object.
func extractDrawerContent(payloadJSON string) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(payloadJSON), &obj); err == nil {
		if v, ok := obj["drawer_content"].(string); ok && v != "" {
			return v
		}
		if v, ok := obj["content"].(string); ok && v != "" {
			return v
		}
	}
	return strings.TrimSpace(payloadJSON)
}

// isNearDuplicate checks the new content against recently-promoted inbox rows
// using the active similarity function. Returns the matching packet_id when a
// duplicate is found. Bounded scan (last 200 promoted) keeps it O(1)-ish.
func isNearDuplicate(db *sql.DB, content string) (bool, string) {
	rows, err := db.Query(
		`SELECT packet_id, drawer_content FROM mesh_knowledge_inbox
		 WHERE status = ? ORDER BY id DESC LIMIT 200`, StatusPromoted)
	if err != nil {
		return false, ""
	}
	defer rows.Close()
	for rows.Next() {
		var pid, existing string
		if rows.Scan(&pid, &existing) != nil {
			continue
		}
		if Similarity(content, existing) >= SimilarityThreshold {
			return true, pid
		}
	}
	return false, ""
}

// ProcessKnowledgePacket runs the full intake pipeline for a verified knowledge
// packet (signature already checked by the caller). It is idempotent: the inbox
// insert is OR-IGNORE and re-processing a packet_id re-evaluates its status.
//
// Karma side-effects:
//   - reject  → -0.1 (poisoning attempt) or -0.05 (low-karma gate), event logged
//   - promote → +0.05 (good contributor)
//   - flag    → no karma change (quarantined for human review)
func ProcessKnowledgePacket(db *sql.DB, pkt Packet) KnowledgeResult {
	content := extractDrawerContent(pkt.PayloadJSON)
	res := KnowledgeResult{PacketID: pkt.PacketID}

	// Always record the packet in the inbox first (shadow), so the audit trail
	// and admin views see it even if it's later dropped.
	_ = IngestKnowledge(db, pkt.PacketID, pkt.OriginPubkey, content)

	// 9-layer filter (L1 signature already passed, L2 freshness, L3 karma, …).
	decisions := RunFilterPipeline(db, pkt, content)
	RecordFilterAudit(db, pkt.PacketID, decisions)
	res.Decisions = decisions

	// Any hard reject → drop + persist status. Karma penalty already applied
	// inside RunFilterPipeline for the karma/injection layers.
	for _, d := range decisions {
		if d.Decision == "reject" {
			_ = PromoteKnowledge(db, pkt.PacketID, StatusDropped)
			res.Status = StatusDropped
			res.Reason = d.Layer + ": " + d.Reason
			return res
		}
	}

	// Near-duplicate of something already promoted → drop as redundant (not a
	// karma penalty; honest peers re-share legitimately).
	if dup, dupID := isNearDuplicate(db, content); dup {
		_ = PromoteKnowledge(db, pkt.PacketID, StatusDropped)
		res.Status = StatusDropped
		res.Duplicate = true
		res.Reason = "near-duplicate of " + dupID
		return res
	}

	// Any flag (suspicious but not fatal) → quarantine for human review.
	for _, d := range decisions {
		if d.Decision == "flag" {
			_ = PromoteKnowledge(db, pkt.PacketID, StatusQuarantine)
			res.Status = StatusQuarantine
			res.Reason = d.Layer + ": " + d.Reason
			return res
		}
	}

	// Clean + novel → promote, reward the contributor's karma.
	_ = PromoteKnowledge(db, pkt.PacketID, StatusPromoted)
	_ = AdjustKarma(db, pkt.OriginPubkey, +0.05, "promoted")
	res.Status = StatusPromoted
	res.Reason = "passed all layers, novel content"
	return res
}
