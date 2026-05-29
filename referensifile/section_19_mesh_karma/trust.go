// Package mesh — trust.go: peer trust score + ban list.
//
// Phase C step 5 anti-poisoning per VISI_FINAL Pilar 3:
//
//	"Trust score per peer (decay kalau push junk).
//	 Manual whitelist Ayah untuk Master-signed tools.
//	 Malicious tool detected → broadcast ban via gossip."
//
// Trust score model:
//   - Per peer (key = peer_id), int 0-100, default 50 (neutral).
//   - +5 on valid signed sync (CRDT, manifest, knowledge).
//   - -10 on bad signature / tamper / poison detected.
//   - -20 on detected malicious payload (caller decides).
//   - <= 0 → auto-ban (added to local ban list).
//   - >= 80 → trusted (knowledge auto-merged tanpa manual review).
//
// Ban list:
//   - Local bans + master bans (signed by Ayah pubkey, hardcoded).
//   - Gossip: tiap peer bisa share ban-list mereka via /v1/mesh/ban-list.
//     Master-signed bans: auto-honor. User-signed bans: weighted by trust
//     score peer that ships the ban (anti-griefing).
//
// Storage: settings DB key-value JSON (sederhana untuk MVP — Phase E3 nanti
// pindah ke SQLite kalau >1k peers).

package mesh

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/flowork/kernel/kernel/settings"
)

const (
	settingsKeyTrustScores = "MESH_TRUST_SCORES"      // map[peer_id]int
	settingsKeyBanList     = "MESH_BAN_LIST"          // []BanEntry JSON
	defaultScore           = 50
	scoreMin               = 0
	scoreMax               = 100
	autoBanThreshold       = 0
	autoTrustThreshold     = 80

	// Score deltas. Caller pakai constant ini supaya consistent.
	ScoreDeltaValidSync     = +5
	ScoreDeltaBadSignature  = -10
	ScoreDeltaMaliciousData = -20
)

// BanEntry — 1 record di ban list.
type BanEntry struct {
	PeerID    string `json:"peer_id"`
	Reason    string `json:"reason"`
	BannedAt  int64  `json:"banned_at"`           // unix sec UTC
	IssuerPID string `json:"issuer_peer_id"`      // who issued the ban
	IssuerSig string `json:"issuer_signature,omitempty"` // base64 ed25519 (gossiped bans only)
}

var (
	trustMu    sync.Mutex
	trustCache map[string]int // peer_id -> score
	banMu      sync.Mutex
	banCache   []BanEntry
)

// loadTrustScores — read from settings DB into cache. Lazy.
func loadTrustScores() map[string]int {
	if trustCache != nil {
		return trustCache
	}
	trustCache = make(map[string]int)
	store := settings.Shared()
	if store == nil {
		return trustCache
	}
	raw, _ := store.Get(settingsKeyTrustScores)
	if strings.TrimSpace(raw) == "" {
		return trustCache
	}
	_ = json.Unmarshal([]byte(raw), &trustCache)
	return trustCache
}

func persistTrustScores() error {
	store := settings.Shared()
	if store == nil {
		return fmt.Errorf("settings unavailable")
	}
	b, err := json.Marshal(trustCache)
	if err != nil {
		return err
	}
	return store.Set(settingsKeyTrustScores, string(b))
}

// AdjustTrust — apply delta to peer's trust score, clamped [0, 100].
// Auto-bans peer kalau score <= 0 dengan reason terkait.
func AdjustTrust(peerID string, delta int, reason string) int {
	if peerID == "" {
		return defaultScore
	}
	trustMu.Lock()
	defer trustMu.Unlock()
	scores := loadTrustScores()
	cur, ok := scores[peerID]
	if !ok {
		cur = defaultScore
	}
	newScore := cur + delta
	if newScore < scoreMin {
		newScore = scoreMin
	}
	if newScore > scoreMax {
		newScore = scoreMax
	}
	scores[peerID] = newScore
	_ = persistTrustScores()

	if newScore <= autoBanThreshold && cur > autoBanThreshold {
		// Auto-ban locally.
		_ = AddLocalBan(peerID, fmt.Sprintf("auto-ban: trust score 0 (%s)", reason))
	}
	return newScore
}

// GetTrust — current trust score (default 50 if unknown peer).
func GetTrust(peerID string) int {
	trustMu.Lock()
	defer trustMu.Unlock()
	scores := loadTrustScores()
	if v, ok := scores[peerID]; ok {
		return v
	}
	return defaultScore
}

// IsTrusted — score >= autoTrustThreshold.
func IsTrusted(peerID string) bool {
	return GetTrust(peerID) >= autoTrustThreshold
}

// === Ban list ===

func loadBanList() []BanEntry {
	if banCache != nil {
		return banCache
	}
	banCache = []BanEntry{}
	store := settings.Shared()
	if store == nil {
		return banCache
	}
	raw, _ := store.Get(settingsKeyBanList)
	if strings.TrimSpace(raw) == "" {
		return banCache
	}
	_ = json.Unmarshal([]byte(raw), &banCache)
	return banCache
}

func persistBanList() error {
	store := settings.Shared()
	if store == nil {
		return fmt.Errorf("settings unavailable")
	}
	b, err := json.Marshal(banCache)
	if err != nil {
		return err
	}
	return store.Set(settingsKeyBanList, string(b))
}

// AddLocalBan — add peer to local ban list (issued by this kernel).
func AddLocalBan(peerID, reason string) error {
	if peerID == "" {
		return fmt.Errorf("ban: empty peer_id")
	}
	banMu.Lock()
	defer banMu.Unlock()
	list := loadBanList()
	for _, b := range list {
		if b.PeerID == peerID {
			return nil // already banned
		}
	}
	banCache = append(list, BanEntry{
		PeerID:    peerID,
		Reason:    reason,
		BannedAt:  time.Now().UTC().Unix(),
		IssuerPID: "self",
	})
	return persistBanList()
}

// IsBanned — true kalau peer ada di ban list.
func IsBanned(peerID string) bool {
	banMu.Lock()
	defer banMu.Unlock()
	for _, b := range loadBanList() {
		if b.PeerID == peerID {
			return true
		}
	}
	return false
}

// AllBans — return snapshot full ban list (sorted by BannedAt desc).
func AllBans() []BanEntry {
	banMu.Lock()
	defer banMu.Unlock()
	list := loadBanList()
	out := make([]BanEntry, len(list))
	copy(out, list)
	sort.Slice(out, func(i, j int) bool {
		return out[i].BannedAt > out[j].BannedAt
	})
	return out
}

// === Cache reset (for tests) ===

// ResetTrustCache clears in-memory trust + ban cache. For tests only.
func ResetTrustCache() {
	trustMu.Lock()
	trustCache = nil
	trustMu.Unlock()
	banMu.Lock()
	banCache = nil
	banMu.Unlock()
}
