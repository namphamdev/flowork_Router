// Package mesh — karma.go: per-peer trust scoring engine (M5).
//
// Replaces legacy trust.go (int 0-100 scale) with float64 [0.0, 1.0].
// Per M-track ARCHITECTURE.md + ide_tambahan.md W-5 CRITICAL resolution.
//
// Karma model:
//   - Per peer (key = pubkey hex), float64 0.0-1.0, default 0.5 (neutral).
//   - VM peers start at 0.3 (reduced trust).
//   - Asymmetric: penalty rate 2x reward rate (recovery harder than fall).
//   - Decay: weekly drift toward 0.5 (anti-stale-trust).
//   - Auto-ban: 5x L1 drop/hour → 24h ban; 10x L4 drop/day → 7d ban.
//   - Endorsement: peer A vouch peer B = karma boost weighted by A's karma.
//
// Storage: settings DB key-value JSON (parity with trust.go legacy).
// Migration path: trust.go int 0-100 → karma float via MigrateLegacyTrust().
package mesh

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/flowork/kernel/kernel/settings"
)

const (
	settingsKeyKarmaScores = "MESH_KARMA_SCORES"   // map[pubkey_hex]KarmaPeer
	settingsKeyKarmaBans   = "MESH_KARMA_BANS"     // map[pubkey_hex]KarmaBan
	settingsKeyKarmaEvents = "MESH_KARMA_EVENTS"   // []KarmaEvent (append-only, capped)

	KarmaDefault        = 0.5
	KarmaDefaultVM      = 0.3
	KarmaMin            = 0.0
	KarmaMax            = 1.0
	KarmaFilterThreshold = 0.5   // L5 filter gate
	KarmaAutoTrust      = 0.8   // auto-merge threshold

	karmaRewardBase  = 0.05
	karmaPenaltyBase = 0.10 // 2x reward (asymmetric)

	karmaEventsCap = 500 // max events per store (LRU)

	// Decay: karma_new = karma * 0.99 + 0.5 * 0.01 (drift toward neutral)
	karmaDecayFactor  = 0.99
	karmaDecayTarget  = 0.5
	karmaDecayWeight  = 0.01
)

// KarmaPeer — per-peer karma state.
type KarmaPeer struct {
	Karma       float64 `json:"karma"`
	FirstSeen   int64   `json:"first_seen"`
	LastSeen    int64   `json:"last_seen"`
	PacketsSent int     `json:"packets_sent"`
	PacketsDrop int     `json:"packets_drop"`
	PacketsPromo int    `json:"packets_promo"`
	IsVirtualized bool  `json:"is_virtualized,omitempty"`
	EndorsedBy  []string `json:"endorsed_by,omitempty"` // pubkey hex list
}

// KarmaBan — temporal or permanent ban entry.
type KarmaBan struct {
	Reason    string `json:"reason"`
	BannedAt  int64  `json:"banned_at"`
	ExpiresAt int64  `json:"expires_at"` // 0 = permanent (manual unban only)
	IssuerPID string `json:"issuer_pid"` // "self" or pubkey hex
}

// KarmaEvent — append-only audit log entry.
type KarmaEvent struct {
	PubKeyHex string  `json:"pubkey"`
	Type      string  `json:"type"`   // "reward" | "penalty" | "ban" | "endorse" | "decay"
	Delta     float64 `json:"delta"`
	Reason    string  `json:"reason"`
	TS        int64   `json:"ts"`
}

// KarmaEngine — orchestrates all karma operations. Thread-safe via mutex.
type KarmaEngine struct {
	mu     sync.Mutex
	scores map[string]*KarmaPeer  // pubkey_hex → state
	bans   map[string]*KarmaBan   // pubkey_hex → ban
	events []KarmaEvent
	loaded bool
}

// NewKarmaEngine creates engine. Data loaded lazily from settings DB on first use.
func NewKarmaEngine() *KarmaEngine {
	return &KarmaEngine{}
}

// load — lazy load from settings DB. Idempotent.
func (k *KarmaEngine) load() {
	if k.loaded {
		return
	}
	k.scores = make(map[string]*KarmaPeer)
	k.bans = make(map[string]*KarmaBan)
	k.events = nil

	store := settings.Shared()
	if store == nil {
		k.loaded = true
		return
	}

	if raw, _ := store.Get(settingsKeyKarmaScores); strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &k.scores)
	}
	if raw, _ := store.Get(settingsKeyKarmaBans); strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &k.bans)
	}
	if raw, _ := store.Get(settingsKeyKarmaEvents); strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &k.events)
	}
	k.loaded = true
}

func (k *KarmaEngine) persist() {
	store := settings.Shared()
	if store == nil {
		return
	}
	if b, err := json.Marshal(k.scores); err == nil {
		_ = store.Set(settingsKeyKarmaScores, string(b))
	}
	if b, err := json.Marshal(k.bans); err == nil {
		_ = store.Set(settingsKeyKarmaBans, string(b))
	}
	// Cap events
	if len(k.events) > karmaEventsCap {
		k.events = k.events[len(k.events)-karmaEventsCap:]
	}
	if b, err := json.Marshal(k.events); err == nil {
		_ = store.Set(settingsKeyKarmaEvents, string(b))
	}
}

// Score — current karma for peer. Default 0.5 if unknown.
func (k *KarmaEngine) Score(pubkey []byte) float64 {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	key := hex.EncodeToString(pubkey)
	if p, ok := k.scores[key]; ok {
		return p.Karma
	}
	return KarmaDefault
}

// GetPeer — full peer karma state. Returns nil if unknown.
func (k *KarmaEngine) GetPeer(pubkey []byte) *KarmaPeer {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	key := hex.EncodeToString(pubkey)
	if p, ok := k.scores[key]; ok {
		cp := *p
		return &cp
	}
	return nil
}

// RegisterPeer — register new peer with initial karma. Idempotent.
func (k *KarmaEngine) RegisterPeer(pubkey []byte, isVirtualized bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	key := hex.EncodeToString(pubkey)
	if _, ok := k.scores[key]; ok {
		// Already registered — update last_seen only
		k.scores[key].LastSeen = time.Now().Unix()
		k.persist()
		return
	}
	initial := KarmaDefault
	if isVirtualized {
		initial = KarmaDefaultVM
	}
	k.scores[key] = &KarmaPeer{
		Karma:         initial,
		FirstSeen:     time.Now().Unix(),
		LastSeen:      time.Now().Unix(),
		IsVirtualized: isVirtualized,
	}
	k.persist()
}

// Reward — increase peer karma. Multiplier scales the base reward.
func (k *KarmaEngine) Reward(pubkey []byte, reason string, multiplier float64) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	key := hex.EncodeToString(pubkey)
	p := k.ensurePeer(key)

	delta := karmaRewardBase * multiplier
	p.Karma = math.Min(KarmaMax, p.Karma+delta)
	p.PacketsPromo++

	k.logEvent(key, "reward", delta, reason)
	k.persist()
	return nil
}

// Penalize — decrease peer karma. Severity scales the base penalty.
func (k *KarmaEngine) Penalize(pubkey []byte, reason string, severity float64) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	key := hex.EncodeToString(pubkey)
	p := k.ensurePeer(key)

	delta := -karmaPenaltyBase * severity
	p.Karma = math.Max(KarmaMin, p.Karma+delta)
	p.PacketsDrop++

	k.logEvent(key, "penalty", delta, reason)

	// Auto-ban checks
	k.checkAutoBan(key, reason)

	k.persist()
	return nil
}

// HardBan — ban peer for duration. Karma set to 0.
func (k *KarmaEngine) HardBan(pubkey []byte, duration time.Duration, reason string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	key := hex.EncodeToString(pubkey)
	p := k.ensurePeer(key)
	p.Karma = KarmaMin

	var expiresAt int64
	if duration > 0 {
		expiresAt = time.Now().Add(duration).Unix()
	}
	k.bans[key] = &KarmaBan{
		Reason:    reason,
		BannedAt:  time.Now().Unix(),
		ExpiresAt: expiresAt,
		IssuerPID: "self",
	}

	k.logEvent(key, "ban", -1.0, reason)
	k.persist()
	return nil
}

// IsBanned — true if peer is currently banned (checks expiry).
func (k *KarmaEngine) IsBanned(pubkey []byte) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	key := hex.EncodeToString(pubkey)
	ban, ok := k.bans[key]
	if !ok {
		return false
	}
	// Check expiry
	if ban.ExpiresAt > 0 && time.Now().Unix() > ban.ExpiresAt {
		delete(k.bans, key) // expired, auto-remove
		k.persist()
		return false
	}
	return true
}

// Endorse — peer A endorses peer B. Reward weighted by endorser's karma.
func (k *KarmaEngine) Endorse(endorserPubkey, targetPubkey []byte, reason string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()

	endorserKey := hex.EncodeToString(endorserPubkey)
	targetKey := hex.EncodeToString(targetPubkey)

	endorser := k.ensurePeer(endorserKey)
	target := k.ensurePeer(targetKey)

	// Weight by endorser karma (low-karma endorsement = no impact)
	weight := endorser.Karma
	if weight < 0.4 {
		return fmt.Errorf("endorser karma %.2f too low to endorse", weight)
	}

	delta := karmaRewardBase * weight
	target.Karma = math.Min(KarmaMax, target.Karma+delta)

	// Track endorsement
	target.EndorsedBy = appendUnique(target.EndorsedBy, endorserKey[:16])

	k.logEvent(targetKey, "endorse", delta, fmt.Sprintf("endorsed by %s: %s", endorserKey[:8], reason))
	k.persist()
	return nil
}

// WeeklyDecay — drift all peer karma toward 0.5. Call from cron.
func (k *KarmaEngine) WeeklyDecay() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()

	for key, p := range k.scores {
		// Skip banned peers
		if _, banned := k.bans[key]; banned {
			continue
		}
		oldKarma := p.Karma
		p.Karma = p.Karma*karmaDecayFactor + karmaDecayTarget*karmaDecayWeight
		k.logEvent(key, "decay", p.Karma-oldKarma, "weekly decay")
	}
	k.persist()
	return nil
}

// ResetPeer — admin reset karma to 0.5 + remove ban. For debug/unban.
func (k *KarmaEngine) ResetPeer(pubkey []byte) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	key := hex.EncodeToString(pubkey)
	if p, ok := k.scores[key]; ok {
		p.Karma = KarmaDefault
	}
	delete(k.bans, key)
	k.logEvent(key, "reward", 0, "admin reset to neutral")
	k.persist()
}

// Events — return last N karma events for peer (or all peers if pubkey nil).
func (k *KarmaEngine) Events(pubkey []byte, limit int) []KarmaEvent {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()

	if limit <= 0 {
		limit = 50
	}

	var result []KarmaEvent
	if pubkey == nil {
		result = k.events
	} else {
		key := hex.EncodeToString(pubkey)
		for _, e := range k.events {
			if e.PubKeyHex == key {
				result = append(result, e)
			}
		}
	}

	if len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

// AllPeers — snapshot all peer karma states.
func (k *KarmaEngine) AllPeers() map[string]*KarmaPeer {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	out := make(map[string]*KarmaPeer, len(k.scores))
	for key, p := range k.scores {
		cp := *p
		out[key] = &cp
	}
	return out
}

// AllBans — snapshot all active bans.
func (k *KarmaEngine) AllBans() map[string]*KarmaBan {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()
	out := make(map[string]*KarmaBan, len(k.bans))
	now := time.Now().Unix()
	for key, b := range k.bans {
		if b.ExpiresAt > 0 && now > b.ExpiresAt {
			continue // expired
		}
		cp := *b
		out[key] = &cp
	}
	return out
}

// ResetForTest — clear all in-memory state + settings DB karma keys.
// Test-only. Required untuk test isolation: tanpa clear settings DB,
// load() di RegisterPeer akan pull stale karma dari prior test run.
func (k *KarmaEngine) ResetForTest() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.scores = nil
	k.bans = nil
	k.events = nil
	k.loaded = false
	// Clear persisted state — settings DB shared singleton across test runs
	if store := settings.Shared(); store != nil {
		_ = store.Set(settingsKeyKarmaScores, "")
		_ = store.Set(settingsKeyKarmaBans, "")
		_ = store.Set(settingsKeyKarmaEvents, "")
	}
}

// --- internal helpers ---

func (k *KarmaEngine) ensurePeer(key string) *KarmaPeer {
	p, ok := k.scores[key]
	if !ok {
		p = &KarmaPeer{
			Karma:     KarmaDefault,
			FirstSeen: time.Now().Unix(),
			LastSeen:  time.Now().Unix(),
		}
		k.scores[key] = p
	}
	p.LastSeen = time.Now().Unix()
	return p
}

func (k *KarmaEngine) logEvent(pubkeyHex, typ string, delta float64, reason string) {
	k.events = append(k.events, KarmaEvent{
		PubKeyHex: pubkeyHex,
		Type:      typ,
		Delta:     delta,
		Reason:    reason,
		TS:        time.Now().Unix(),
	})
}

func (k *KarmaEngine) checkAutoBan(pubkeyHex, lastReason string) {
	now := time.Now()

	if strings.HasPrefix(lastReason, "L1") {
		count := k.countRecentEvents(pubkeyHex, "L1", now.Add(-time.Hour))
		if count >= 5 {
			k.bans[pubkeyHex] = &KarmaBan{
				Reason:    "auto: 5x L1 drop/hour",
				BannedAt:  now.Unix(),
				ExpiresAt: now.Add(24 * time.Hour).Unix(),
				IssuerPID: "self",
			}
			k.scores[pubkeyHex].Karma = KarmaMin
			k.logEvent(pubkeyHex, "ban", -1.0, "auto-ban: 5x L1 drop/hour")
		}
	}
	if strings.HasPrefix(lastReason, "L4") {
		count := k.countRecentEvents(pubkeyHex, "L4", now.Add(-24*time.Hour))
		if count >= 10 {
			k.bans[pubkeyHex] = &KarmaBan{
				Reason:    "auto: 10x L4 drop/day",
				BannedAt:  now.Unix(),
				ExpiresAt: now.Add(7 * 24 * time.Hour).Unix(),
				IssuerPID: "self",
			}
			k.scores[pubkeyHex].Karma = KarmaMin
			k.logEvent(pubkeyHex, "ban", -1.0, "auto-ban: 10x L4 drop/day")
		}
	}
}

func (k *KarmaEngine) countRecentEvents(pubkeyHex, reasonPrefix string, since time.Time) int {
	sinceUnix := since.Unix()
	count := 0
	for _, e := range k.events {
		if e.PubKeyHex == pubkeyHex && e.Type == "penalty" &&
			strings.HasPrefix(e.Reason, reasonPrefix) && e.TS >= sinceUnix {
			count++
		}
	}
	return count
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

// --- Legacy migration (trust.go int 0-100 → karma float 0-1) ---

// MigrateLegacyTrust reads old trust.go data (MESH_TRUST_SCORES int 0-100 +
// MESH_BAN_LIST) and converts to new karma format. Idempotent — skips if
// karma data already exists.
//
// Per W-5 CRITICAL (ide_tambahan.md): this MUST run before any M-track
// mesh operation.
func (k *KarmaEngine) MigrateLegacyTrust() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load()

	// Skip if karma data already populated
	if len(k.scores) > 0 {
		return nil
	}

	store := settings.Shared()
	if store == nil {
		return nil
	}

	// Read legacy trust scores (int 0-100)
	raw, _ := store.Get(settingsKeyTrustScores) // "MESH_TRUST_SCORES"
	if strings.TrimSpace(raw) == "" {
		return nil // no legacy data
	}

	var legacyScores map[string]int
	if err := json.Unmarshal([]byte(raw), &legacyScores); err != nil {
		return fmt.Errorf("migrate legacy trust: unmarshal scores: %w", err)
	}

	// Convert int 0-100 → float 0.0-1.0
	for peerID, score := range legacyScores {
		k.scores[peerID] = &KarmaPeer{
			Karma:     float64(score) / 100.0,
			FirstSeen: time.Now().Unix(),
			LastSeen:  time.Now().Unix(),
		}
	}

	// Read legacy ban list
	rawBans, _ := store.Get(settingsKeyBanList) // "MESH_BAN_LIST"
	if strings.TrimSpace(rawBans) != "" {
		var legacyBans []BanEntry
		if err := json.Unmarshal([]byte(rawBans), &legacyBans); err == nil {
			for _, b := range legacyBans {
				k.bans[b.PeerID] = &KarmaBan{
					Reason:    b.Reason,
					BannedAt:  b.BannedAt,
					ExpiresAt: 0, // legacy bans = permanent until manual unban
					IssuerPID: b.IssuerPID,
				}
			}
		}
	}

	k.logEvent("system", "reward", 0, fmt.Sprintf(
		"migrated %d legacy trust scores + %d bans to karma engine",
		len(legacyScores), len(k.bans)))

	k.persist()
	return nil
}
