// Package mesh — packet.go: KnowledgePacket type definition (M3).
//
// KnowledgePacket = unit data yang ditukar antar peer via gossip protocol
// (M6). Setiap packet di-sign oleh author (ed25519 self-issued license dari
// M1), masuk ke filter pipeline 9-lapis (M4), eventually promoted ke main
// brain kalau lulus consensus + cosine validate.
//
// Per ARCHITECTURE.md (post-amendment v1):
//   - Type: "drawer" | "kg_triple" | "cached_reasoning" | "v4_lora_delta" |
//           "sunset_notice" | "emergency_alert"
//   - Sign target = sha256(CanonicalBytes(p)) — DETERMINISTIC, bukan
//     json.Marshal (W-2 fix). Implementasi canonical di canonical.go (M6).
//   - Append-only: status row di peer_packets ngga delete, FQP-12.
//   - Hop count cap 7 (anti-flood, mirip TTL TCP).
//
// File ini cuma type def. Persistence ada di mesh_db.go. Filter ada di
// filter.go (M4). Sign/verify canonical ada di canonical.go (M6).

package mesh

import (
	"time"

	"github.com/google/uuid"
)

// PacketType — enum string untuk type field.
const (
	PacketTypeDrawer          = "drawer"
	PacketTypeKGTriple        = "kg_triple"
	PacketTypeCachedReasoning = "cached_reasoning"
	PacketTypeV4LoraDelta     = "v4_lora_delta"
	PacketTypeSunsetNotice    = "sunset_notice"
	PacketTypeEmergencyAlert  = "emergency_alert"
)

// FilterStatus — status hasil filter pipeline 9-lapis.
const (
	FilterStatusShadow      = "shadow"      // L6 lulus, masuk shadow zone
	FilterStatusQuarantine  = "quarantine"  // I-1: visible-but-labeled, awaiting user feedback
	FilterStatusPromoted    = "promoted"    // L9 lulus, masuk main brain
	FilterStatusInvalidated = "invalidated" // post-promote ke-revoke (valid_to set)
	FilterStatusExpired     = "expired"     // I-4: TTL expires_at lewat
	FilterStatusDropped     = "dropped"     // L1-L5 reject (audit trail tetap)
)

// HopCountMax = max hop traversal sebelum packet di-drop. Anti-flood.
const HopCountMax = 7

// KnowledgePacket — unit data tukar otak antar peer.
//
// Sign target (deterministic):
//
//	sha256(CanonicalBytes(packet))
//
// CanonicalBytes layout (per W-2 hash-then-sign fix di M6):
//
//	type || 0x00 || payload || 0x00 || timestamp_unixnano_be || 0x00 || parent_id
//
// Field order di canonical bytes TETAP, ngga peduli platform.
type KnowledgePacket struct {
	ID           uuid.UUID `json:"id"`
	Type         string    `json:"type"`           // PacketTypeXxx
	Payload      []byte    `json:"payload"`        // encoded content (per type)
	AuthorPubKey []byte    `json:"author_pubkey"`  // ed25519 public key (32 bytes)
	LicenseID    string    `json:"license_id"`     // hardware-bound machine fingerprint (M1)
	Signature    []byte    `json:"signature"`      // ed25519(sha256(CanonicalBytes))
	ParentID     string    `json:"parent_id,omitempty"` // chain proof — packet sebelumnya dari author yg sama
	Amplitude    float64   `json:"amplitude"`      // self-rated confidence 0..1
	Timestamp    time.Time `json:"timestamp"`
	HopCount     int       `json:"hop_count"`      // berapa peer udah lewat (cap HopCountMax)
	ExpiresAt    *time.Time `json:"expires_at,omitempty"` // I-4 TTL: nil = never expire
}

// PeerInfo — entry di peer_registry (per peer trust state).
//
// Storage:
//   - karma + banned_until: M3 schema kolom (sekarang stub, M5 Step 5 migrate
//     dari settings DB key-value MESH_KARMA_SCORES ke sini)
//   - first/last seen + counters: M3 schema kolom utama
type PeerInfo struct {
	PubKey        []byte     `json:"pubkey"`          // ed25519 32 bytes
	LicenseID     string     `json:"license_id"`
	FirstSeen     time.Time  `json:"first_seen"`
	LastSeen      time.Time  `json:"last_seen"`
	Karma         float64    `json:"karma"`           // 0.0-1.0, populated post-M5 migration
	PacketsSent   int        `json:"packets_sent"`
	PacketsDrop   int        `json:"packets_drop"`
	PacketsPromo  int        `json:"packets_promo"`
	EndorsedBy    []string   `json:"endorsed_by,omitempty"` // pubkey hex list
	BannedUntil   *time.Time `json:"banned_until,omitempty"`
	IsVirtualized bool       `json:"is_virtualized"`
}

// ShadowEntry — row di shadow_drawers (zona karantina antara L6 dan L9).
type ShadowEntry struct {
	PacketID            string    `json:"packet_id"`
	DrawerContent       string    `json:"drawer_content"`
	CosineScore         float64   `json:"cosine_score"`            // L7 result, -1 = belum di-validate
	ConsensusCount      int       `json:"consensus_count"`         // L8: jumlah peer independen submit fakta serupa
	ShadowSince         time.Time `json:"shadow_since"`
	ScheduledPromoteAt  time.Time `json:"scheduled_promote_at"`    // earliest eligible to promote
	UserFeedback        string    `json:"user_feedback,omitempty"` // I-1: "thumbs_up" | "thumbs_down" | ""
}

// MeshStats — agregat untuk endpoint /v1/mesh/stats.
type MeshStats struct {
	Packets struct {
		Total       int `json:"total"`
		Shadow      int `json:"shadow"`
		Quarantine  int `json:"quarantine"`
		Promoted    int `json:"promoted"`
		Invalidated int `json:"invalidated"`
		Expired     int `json:"expired"`
		Dropped     int `json:"dropped"`
	} `json:"packets"`
	Peers struct {
		Total      int     `json:"total"`
		Active24h  int     `json:"active_24h"`
		Banned     int     `json:"banned"`
		Virtualized int    `json:"virtualized"`
		AvgKarma   float64 `json:"avg_karma"`
	} `json:"peers"`
}

// IsValidStatus return true kalau s ada di enum FilterStatusXxx.
func IsValidStatus(s string) bool {
	switch s {
	case FilterStatusShadow, FilterStatusQuarantine, FilterStatusPromoted,
		FilterStatusInvalidated, FilterStatusExpired, FilterStatusDropped:
		return true
	}
	return false
}

// IsValidPacketType return true kalau t ada di enum PacketTypeXxx.
func IsValidPacketType(t string) bool {
	switch t {
	case PacketTypeDrawer, PacketTypeKGTriple, PacketTypeCachedReasoning,
		PacketTypeV4LoraDelta, PacketTypeSunsetNotice, PacketTypeEmergencyAlert:
		return true
	}
	return false
}
