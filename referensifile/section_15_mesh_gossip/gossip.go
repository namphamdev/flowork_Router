// Package mesh — gossip.go: M6 push/pull protocol + bandwidth limit +
// emergency 2-of-3 BFT broadcast (per AMENDMENTS-V1 I-3).
//
// Push: peer A POST /v1/mesh/sync/push with signed batch → kernel filters
// each packet via M4 pipeline → store result. Anti-flood: 1MB/peer/hour
// + 100 packet/batch cap + hop_count cap 7.
//
// Pull: peer B GET /v1/mesh/sync/pull?since=<unix> → return promoted
// packets sejak timestamp + hop_count++ saat forward. Cap 100/response.
//
// Cron auto-gossip every 5 min: 2 top-karma + 1 random (W-4 echo
// chamber fix). Bandwidth tracked per peer.
//
// Emergency: PacketTypeEmergencyAlert dengan 2-of-3 master sig bypass
// filter (skip L3-L8) — tetap L1 verify wajib + L2 master pubkey check.

package mesh

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// PushBatch payload struktur saat peer push ke kita.
type PushBatch struct {
	FromPubKey []byte             `json:"from_pubkey"`
	Signature  []byte             `json:"signature"`  // ed25519 sign(canonical of packets || timestamp)
	Timestamp  int64              `json:"timestamp"`  // unix nano
	Packets    []KnowledgePacket  `json:"packets"`
}

// PushResult per-batch summary.
type PushResult struct {
	Accepted  int      `json:"accepted"`
	Rejected  int      `json:"rejected"`
	Throttled int      `json:"throttled"`
	Reasons   []string `json:"reasons,omitempty"`
}

// MasterPubKeys — 3 master pubkey hardcoded untuk I-3 emergency 2-of-3 BFT.
//
// Real values di-set saat build via `cmd/gen-license-key` atau init helper.
// Default = 3 zero keys → emergency disabled (safe default kalau belum
// configured).
var MasterPubKeys = [3]ed25519.PublicKey{
	make(ed25519.PublicKey, ed25519.PublicKeySize),
	make(ed25519.PublicKey, ed25519.PublicKeySize),
	make(ed25519.PublicKey, ed25519.PublicKeySize),
}

// SetMasterPubKeys override default (test + future build-time inject).
// Caller wajib pass 3 valid 32-byte ed25519 pubkeys.
func SetMasterPubKeys(keys [3]ed25519.PublicKey) {
	MasterPubKeys = keys
}

// EmergencyMultiSig — extra field di KnowledgePacket Payload untuk type=emergency_alert.
//
// Layout: JSON encoded {sigs: [3 signatures, 64 bytes each base hex]}
// dimana minimal 2 signature WAJIB valid (sign sha256 of payload header).
type EmergencyMultiSig struct {
	Sigs []string `json:"sigs"` // hex-encoded 64-byte signatures, max 3
}

// ---------- Gossip ----------

// Gossip orchestrator gossip cycles.
type Gossip struct {
	store     *MeshDB
	karma     *KarmaEngine
	filter    *FilterPipeline
	bandwidth *BandwidthLimiter
	selfPub   []byte
	signFn    func([]byte) []byte // sign batch dengan local priv key
	mu        sync.Mutex
	syncedTo  map[string]int64    // pubkey hex → last_synced_unix
	pulledFrom map[string]int64
}

// NewGossip construct.
func NewGossip(store *MeshDB, karma *KarmaEngine, filter *FilterPipeline, selfPub []byte, signFn func([]byte) []byte) *Gossip {
	return &Gossip{
		store:      store,
		karma:      karma,
		filter:     filter,
		bandwidth:  NewBandwidthLimiter(),
		selfPub:    selfPub,
		signFn:     signFn,
		syncedTo:   map[string]int64{},
		pulledFrom: map[string]int64{},
	}
}

// MaxPacketPayloadBytes — Sprint 3.5e (BUG-W8 fix): per-packet size cap
// untuk anti DoS via single huge packet. 1MB cukup untuk knowledge packet
// content + metadata; mesin malicious yang inject 100MB payload reject di
// gate ini sebelum bandwidth check.
const MaxPacketPayloadBytes = 1 << 20 // 1MB

// MaxBatchTotalBytes — circuit breaker untuk total batch (anti many-small-packets).
const MaxBatchTotalBytes = 10 << 20 // 10MB

// HandlePush proses incoming PushBatch. Verify peer signature, apply filter
// per-packet, return summary.
func (g *Gossip) HandlePush(batch PushBatch) (PushResult, error) {
	if len(batch.Packets) > 100 {
		return PushResult{}, errors.New("batch too large (>100)")
	}

	// Verify peer signature on canonical batch bytes
	canonical := canonicalBatchBytes(batch)
	hash := sha256.Sum256(canonical)
	if !ed25519.Verify(ed25519.PublicKey(batch.FromPubKey), hash[:], batch.Signature) {
		return PushResult{}, errors.New("batch signature invalid")
	}

	// BUG-W8 fix Sprint 3.5e: per-packet + total batch size gate sebelum
	// bandwidth tracker. Anti DoS via single 100MB packet atau 100×1MB packet
	// yang lolos `len > 100` check tapi blow up memory.
	totalBytes := 0
	for _, p := range batch.Packets {
		if len(p.Payload) > MaxPacketPayloadBytes {
			return PushResult{}, fmt.Errorf("packet payload too large: %d bytes (max %d)", len(p.Payload), MaxPacketPayloadBytes)
		}
		totalBytes += len(p.Payload)
		if totalBytes > MaxBatchTotalBytes {
			return PushResult{}, fmt.Errorf("batch total payload too large: >%d bytes (cap %d)", totalBytes, MaxBatchTotalBytes)
		}
	}
	if !g.bandwidth.Allow(batch.FromPubKey, totalBytes) {
		return PushResult{}, errors.New("bandwidth quota exceeded")
	}

	res := PushResult{}
	for _, p := range batch.Packets {
		// Hop count enforcement
		if p.HopCount > HopCountMax {
			res.Rejected++
			res.Reasons = append(res.Reasons, "hop>max")
			continue
		}
		// Loop prevention: dedup
		if exists, _ := g.store.PacketExists(p.ID.String()); exists {
			// Increment consensus count for shadow packets seen by another peer
			_ = g.store.IncrementShadowConsensus(p.ID.String())
			continue
		}

		// Special: emergency alert 2-of-3 BFT bypass filter
		if p.Type == PacketTypeEmergencyAlert {
			if g.verifyEmergency2of3(p) {
				_ = g.store.InsertPacket(p, FilterStatusPromoted)
				res.Accepted++
				continue
			}
			res.Rejected++
			res.Reasons = append(res.Reasons, "emergency 2-of-3 fail")
			continue
		}

		// Normal filter pipeline
		out := g.filter.Process(p)
		switch out.Decision {
		case DecisionDrop, DecisionThrottle:
			res.Rejected++
		default:
			res.Accepted++
		}
	}
	return res, nil
}

// HandlePull return promoted packets sejak `since` (unix). Increment hop_count.
// Cap 100 packets per response.
func (g *Gossip) HandlePull(fromPubKey []byte, since int64) ([]KnowledgePacket, error) {
	peer, err := g.store.GetPeer(fromPubKey)
	if err != nil {
		return nil, fmt.Errorf("unknown peer: %w", err)
	}
	if peer.BannedUntil != nil && time.Now().Before(*peer.BannedUntil) {
		return nil, errors.New("peer banned")
	}
	pkts, err := g.store.ListPromotedSince(since, 100)
	if err != nil {
		return nil, err
	}
	for i := range pkts {
		pkts[i].HopCount++
	}
	return pkts, nil
}

// RoundRobin — auto-gossip cycle: 2 top-karma + 1 random peer (W-4).
func (g *Gossip) RoundRobin() error {
	peers, err := g.store.ListPeers(100)
	if err != nil {
		return err
	}
	if len(peers) == 0 {
		return nil
	}

	// Sort by karma desc → pick top 2
	sorted := make([]PeerInfo, len(peers))
	copy(sorted, peers)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Karma > sorted[i].Karma {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	selected := []PeerInfo{}
	for i := 0; i < 2 && i < len(sorted); i++ {
		selected = append(selected, sorted[i])
	}
	// 1 random peer with karma >= 0.4 (W-4 anti echo chamber)
	var pool []PeerInfo
	for _, p := range peers {
		if p.Karma >= 0.4 {
			alreadyIn := false
			for _, s := range selected {
				if string(s.PubKey) == string(p.PubKey) {
					alreadyIn = true
					break
				}
			}
			if !alreadyIn {
				pool = append(pool, p)
			}
		}
	}
	if len(pool) > 0 {
		// nolint:gosec — non-crypto random for selection diversity
		selected = append(selected, pool[rand.Intn(len(pool))])
	}

	// Push delta + pull from each selected peer
	for _, peer := range selected {
		_ = g.pushDeltaToPeer(peer)
		_ = g.pullDeltaFromPeer(peer)
	}
	return nil
}

func (g *Gossip) pushDeltaToPeer(peer PeerInfo) error {
	g.mu.Lock()
	since := g.syncedTo[hex.EncodeToString(peer.PubKey)]
	g.mu.Unlock()
	delta, err := g.store.ListPromotedSince(since, 50)
	if err != nil || len(delta) == 0 {
		return err
	}
	// Real push via HTTP would happen here. For now, mark synced (cron skeleton).
	g.mu.Lock()
	g.syncedTo[hex.EncodeToString(peer.PubKey)] = time.Now().Unix()
	g.mu.Unlock()
	return nil
}

func (g *Gossip) pullDeltaFromPeer(peer PeerInfo) error {
	g.mu.Lock()
	g.pulledFrom[hex.EncodeToString(peer.PubKey)] = time.Now().Unix()
	g.mu.Unlock()
	return nil
}

// emergencyCanonicalBytes — hash target untuk emergency packet (I-3).
//
// EXCLUDES ParentID + Signature dari hash supaya extra sigs bisa stored di
// ParentID tanpa invalidate hash. Format:
//
//	type || 0x00 || payload || 0x00 || timestamp_unixnano_be
func emergencyCanonicalBytes(p KnowledgePacket) []byte {
	var buf []byte
	buf = append(buf, []byte(p.Type)...)
	buf = append(buf, 0x00)
	buf = append(buf, p.Payload...)
	buf = append(buf, 0x00)
	var ts [8]byte
	for i := 0; i < 8; i++ {
		ts[i] = byte(uint64(p.Timestamp.UTC().UnixNano()) >> (8 * (7 - i)))
	}
	buf = append(buf, ts[:]...)
	return buf
}

// verifyEmergency2of3 — I-3 emergency alert harus punya ≥2 valid sigs dari
// 3 hardcoded MasterPubKeys.
//
// Hash target: emergencyCanonicalBytes (excludes ParentID supaya extra sigs
// bisa stored di ParentID format "sig2hex,sig3hex").
func (g *Gossip) verifyEmergency2of3(p KnowledgePacket) bool {
	canonical := emergencyCanonicalBytes(p)
	hash := sha256.Sum256(canonical)

	validCount := 0
	for _, mk := range MasterPubKeys {
		if isZeroKey(mk) {
			continue
		}
		if ed25519.Verify(mk, hash[:], p.Signature) {
			validCount++
		}
	}
	// Parse additional sigs from ParentID field (comma-sep hex)
	if p.ParentID != "" {
		// Format: "sig2hex,sig3hex" or "sig2hex"
		parts := splitComma(p.ParentID)
		for _, sigHex := range parts {
			sig, err := hex.DecodeString(sigHex)
			if err != nil || len(sig) != ed25519.SignatureSize {
				continue
			}
			for _, mk := range MasterPubKeys {
				if isZeroKey(mk) {
					continue
				}
				if ed25519.Verify(mk, hash[:], sig) {
					validCount++
					break
				}
			}
		}
	}
	return validCount >= 2
}

func canonicalBatchBytes(b PushBatch) []byte {
	var buf []byte
	buf = append(buf, b.FromPubKey...)
	buf = append(buf, 0x00)
	for _, p := range b.Packets {
		buf = append(buf, []byte(p.ID.String())...)
		buf = append(buf, 0x00)
	}
	var ts [8]byte
	for i := 0; i < 8; i++ {
		ts[i] = byte(b.Timestamp >> (8 * (7 - i)))
	}
	buf = append(buf, ts[:]...)
	return buf
}

func isZeroKey(k ed25519.PublicKey) bool {
	for _, b := range k {
		if b != 0 {
			return false
		}
	}
	return true
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// ---------- BandwidthLimiter ----------

// BandwidthLimiter per-peer bytes/hour cap.
type BandwidthLimiter struct {
	mu        sync.Mutex
	perPeer   map[string]*bytesBucket
	perPeerCap int64 // 1MB/hour default
}

type bytesBucket struct {
	bytes       int64
	windowStart time.Time
}

// NewBandwidthLimiter construct.
func NewBandwidthLimiter() *BandwidthLimiter {
	return &BandwidthLimiter{
		perPeer:    make(map[string]*bytesBucket),
		perPeerCap: 1 << 20, // 1MB
	}
}

// Allow check + consume bytes. Return false kalau exceed.
func (bl *BandwidthLimiter) Allow(pubkey []byte, bytes int) bool {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	key := hex.EncodeToString(pubkey)
	b, ok := bl.perPeer[key]
	if !ok {
		b = &bytesBucket{windowStart: time.Now()}
		bl.perPeer[key] = b
	}
	if time.Since(b.windowStart) > time.Hour {
		b.bytes = 0
		b.windowStart = time.Now()
	}
	if b.bytes+int64(bytes) > bl.perPeerCap {
		return false
	}
	b.bytes += int64(bytes)
	return true
}
