// Package mesh — filter.go: M4 filter pipeline 9-lapis (anti-poisoning).
//
// Per ARCHITECTURE.md + AMENDMENTS-V1: setiap incoming KnowledgePacket
// dari peer mesh WAJIB lewat 9-lapis filter sebelum eligible promote ke
// main brain. Tidak ada bypass.
//
//   L1 SIGNATURE  → ed25519 verify canonical bytes hash
//   L2 LICENSE    → license_id match peer registry + revocation check
//   L3 RATE LIMIT → token bucket per peer (100 packet/menit)
//   L4 BLACKLIST  → regex content scan (PII, secret, prompt-inject)
//   L5 KARMA      → peer karma >= 0.5 (0.6 untuk VM, W-1)
//   L6 SHADOW     → masuk shadow zone, BELUM main brain
//   L7 COSINE     → TF-IDF similarity vs local centroid (M4.5)
//   L8 CONSENSUS  → multi-source ≥ minPeers (adaptive W-7)
//   L9 PROMOTE    → eligible promote ke main drawer post-cooldown
//
// Plus I-1 Quarantine zone: cosine ≥ 0.5 + consensus < minPeers =
// visible-but-labeled untuk user feedback. Thumbs up = fast-track promote.

package mesh

import (
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ---------- FilterResult ----------

// FilterDecision enum hasil pipeline.
type FilterDecision string

const (
	DecisionDrop             FilterDecision = "drop"
	DecisionThrottle         FilterDecision = "throttle"
	DecisionShadow           FilterDecision = "shadow"
	DecisionQuarantine       FilterDecision = "quarantine"
	DecisionPromoteEligible  FilterDecision = "promote_eligible"
	DecisionEmergencyPromote FilterDecision = "emergency_promote" // I-3 master 2-of-3
)

// FilterResult per-packet outcome dengan layer + reason untuk audit.
type FilterResult struct {
	Decision    FilterDecision `json:"decision"`
	Layer       string         `json:"layer"`
	Reason      string         `json:"reason"`
	CosineScore float64        `json:"cosine_score,omitempty"`
	Audit       string         `json:"audit"`
}

// ---------- FilterPipeline ----------

// FilterPipeline orchestrator 9-lapis. Construct via NewFilterPipeline.
type FilterPipeline struct {
	store        *MeshDB
	karma        *KarmaEngine
	cosine       *CosineEngine
	rateLimit    *RateLimit
	blacklist    *Blacklist
	consensus    *ConsensusGate
}

// NewFilterPipeline construct dengan dependency. cosine boleh nil (akan
// graceful degrade ke neutral 0.5).
func NewFilterPipeline(store *MeshDB, karma *KarmaEngine, cosine *CosineEngine) *FilterPipeline {
	return &FilterPipeline{
		store:     store,
		karma:     karma,
		cosine:    cosine,
		rateLimit: NewRateLimit(),
		blacklist: NewBlacklist(),
		consensus: NewConsensusGate(),
	}
}

// Process run packet through 9-lapis pipeline. Return FilterResult.
//
// Side effects:
//   - DB writes: peer_packets row (always, audit trail), shadow_drawers (kalau lulus L6)
//   - Karma updates: penalty kalau drop di L1/L4, reward kalau promote
//   - Loop prevention: caller wajib check PacketExists dulu
func (f *FilterPipeline) Process(p KnowledgePacket) FilterResult {
	auditLines := []string{
		fmt.Sprintf("filter start: id=%s type=%s author=%s hop=%d",
			p.ID.String(), p.Type, hex.EncodeToString(p.AuthorPubKey)[:8], p.HopCount),
	}

	// L1: Signature
	if !VerifyPacketSignature(p, p.AuthorPubKey) {
		f.penalize(p.AuthorPubKey, "L1 signature invalid", 3.0)
		auditLines = append(auditLines, "L1 DROP: signature invalid")
		f.persistDropped(p, "L1: signature invalid", strings.Join(auditLines, "\n"))
		return FilterResult{Decision: DecisionDrop, Layer: "L1", Reason: "signature invalid", Audit: strings.Join(auditLines, "\n")}
	}
	auditLines = append(auditLines, "L1 PASS: signature valid")

	// L2: License + revocation check
	pubkeyHex := hex.EncodeToString(p.AuthorPubKey)
	if IsRevoked(pubkeyHex) {
		f.penalize(p.AuthorPubKey, "L2 revoked pubkey", 3.0)
		auditLines = append(auditLines, "L2 DROP: pubkey revoked")
		f.persistDropped(p, "L2: pubkey revoked", strings.Join(auditLines, "\n"))
		return FilterResult{Decision: DecisionDrop, Layer: "L2", Reason: "pubkey revoked", Audit: strings.Join(auditLines, "\n")}
	}
	peer, peerErr := f.store.GetPeer(p.AuthorPubKey)
	if peerErr != nil {
		// Unknown peer — register dengan default karma (let karma engine handle)
		// Note: in production, M2 discovery / M6 gossip should pre-populate registry.
		// For first-encounter packets, treat as low-trust until karma builds.
		auditLines = append(auditLines, fmt.Sprintf("L2 unknown peer (auto-register, low trust): %v", peerErr))
		_ = f.store.UpsertPeer(PeerInfo{
			PubKey:    p.AuthorPubKey,
			LicenseID: p.LicenseID,
			Karma:     0.5, // neutral — ngga otomatis trust
		})
		peer, _ = f.store.GetPeer(p.AuthorPubKey)
	}
	if peer != nil && peer.LicenseID != "" && peer.LicenseID != p.LicenseID {
		f.penalize(p.AuthorPubKey, "L2 license_id mismatch", 2.0)
		auditLines = append(auditLines, "L2 DROP: license_id mismatch")
		f.persistDropped(p, "L2: license_id mismatch", strings.Join(auditLines, "\n"))
		return FilterResult{Decision: DecisionDrop, Layer: "L2", Reason: "license_id mismatch", Audit: strings.Join(auditLines, "\n")}
	}
	if peer != nil && peer.BannedUntil != nil && time.Now().Before(*peer.BannedUntil) {
		auditLines = append(auditLines, "L2 DROP: peer banned")
		f.persistDropped(p, "L2: peer banned", strings.Join(auditLines, "\n"))
		return FilterResult{Decision: DecisionDrop, Layer: "L2", Reason: "peer banned", Audit: strings.Join(auditLines, "\n")}
	}
	auditLines = append(auditLines, "L2 PASS: license valid, not revoked, not banned")

	// L3: Rate limit
	if !f.rateLimit.Allow(p.AuthorPubKey) {
		auditLines = append(auditLines, "L3 THROTTLE: rate exceed")
		// Throttle = ngga insert ke peer_packets (anti-flood DB), cuma return.
		return FilterResult{Decision: DecisionThrottle, Layer: "L3", Reason: "rate limit exceeded", Audit: strings.Join(auditLines, "\n")}
	}
	auditLines = append(auditLines, "L3 PASS: rate ok")

	// L4: Content blacklist
	if reason, blocked := f.blacklist.Scan(p.Payload); blocked {
		f.penalize(p.AuthorPubKey, "L4 "+reason, 2.0)
		auditLines = append(auditLines, "L4 DROP: "+reason)
		f.persistDropped(p, "L4: "+reason, strings.Join(auditLines, "\n"))
		return FilterResult{Decision: DecisionDrop, Layer: "L4", Reason: reason, Audit: strings.Join(auditLines, "\n")}
	}
	auditLines = append(auditLines, "L4 PASS: no blacklist match")

	// L5: Karma threshold
	karmaScore := 0.5
	if f.karma != nil {
		karmaScore = f.karma.Score(p.AuthorPubKey)
	}
	threshold := KarmaFilterThreshold // 0.5
	if peer != nil && peer.IsVirtualized {
		threshold = 0.6 // VM stricter
	}
	if karmaScore < threshold {
		auditLines = append(auditLines, fmt.Sprintf("L5 DROP: karma %.2f < %.2f", karmaScore, threshold))
		f.persistDropped(p, fmt.Sprintf("L5: karma %.2f<%.2f", karmaScore, threshold), strings.Join(auditLines, "\n"))
		return FilterResult{Decision: DecisionDrop, Layer: "L5", Reason: "karma below threshold", Audit: strings.Join(auditLines, "\n")}
	}
	auditLines = append(auditLines, fmt.Sprintf("L5 PASS: karma %.2f", karmaScore))

	// L6: Shadow insert
	if err := f.store.InsertPacket(p, FilterStatusShadow); err != nil {
		auditLines = append(auditLines, "L6 ERROR: insert failed: "+err.Error())
		return FilterResult{Decision: DecisionDrop, Layer: "L6", Reason: "store error", Audit: strings.Join(auditLines, "\n")}
	}
	drawerContent := string(p.Payload) // assume drawer text payload; for binary types skip cosine
	if p.Type == PacketTypeDrawer {
		_ = f.store.InsertShadow(p.ID.String(), drawerContent, 24*time.Hour)
	}
	auditLines = append(auditLines, "L6 SHADOW: inserted to peer_packets + shadow_drawers")

	// L7: Cosine validate (drawer type only)
	cosineScore := 0.5 // neutral default
	verdict := CosineVerdictNeutral
	if p.Type == PacketTypeDrawer && f.cosine != nil {
		cosineScore = f.cosine.CosineForBytes(p.Payload)
		verdict = ClassifyCosine(cosineScore)
		_ = f.store.UpdateShadowCosine(p.ID.String(), cosineScore)
		auditLines = append(auditLines, fmt.Sprintf("L7 COSINE: %.4f (%s)", cosineScore, verdict))
	} else {
		auditLines = append(auditLines, "L7 SKIP: non-drawer type or cosine disabled")
	}

	// L8: Consensus check (adaptive — W-7)
	consensusEligible := f.consensus.Eligible(f.store, p.ID.String(), verdict)
	auditLines = append(auditLines, fmt.Sprintf("L8 CONSENSUS: eligible=%v", consensusEligible))

	// I-1 Quarantine path: cosine ≥ 0.5 + consensus < minPeers = visible-but-labeled
	if !consensusEligible && cosineScore >= 0.5 && p.Type == PacketTypeDrawer {
		_ = f.store.UpdatePacketStatus(p.ID.String(), FilterStatusQuarantine, "moved to quarantine zone (cosine ok, awaiting consensus or feedback)")
		auditLines = append(auditLines, "QUARANTINE: visible-but-labeled, awaiting feedback")
		return FilterResult{Decision: DecisionQuarantine, Layer: "I-1", Reason: "consensus pending, cosine relevant", CosineScore: cosineScore, Audit: strings.Join(auditLines, "\n")}
	}

	if !consensusEligible {
		auditLines = append(auditLines, "FINAL: shadow (consensus not eligible)")
		_ = f.store.UpdatePacketStatus(p.ID.String(), FilterStatusShadow, "awaiting consensus + cooldown")
		return FilterResult{Decision: DecisionShadow, Layer: "L8", Reason: "consensus pending", CosineScore: cosineScore, Audit: strings.Join(auditLines, "\n")}
	}

	// L9: Promote eligible — caller (gossip cron) yang actual promote.
	auditLines = append(auditLines, "L9 PROMOTE_ELIGIBLE: ready for cron promote")
	return FilterResult{Decision: DecisionPromoteEligible, Layer: "L9", Reason: "all checks pass", CosineScore: cosineScore, Audit: strings.Join(auditLines, "\n")}
}

// PromotePacket actual transition shadow → promoted. Called by caller
// (gossip cron / endpoint) setelah Process return DecisionPromoteEligible.
//
// Updates:
//   - peer_packets.filter_status = promoted
//   - peer karma reward
//   - audit log appended
func (f *FilterPipeline) PromotePacket(packetID string, authorPubKey []byte) error {
	if err := f.store.UpdatePacketStatus(packetID, FilterStatusPromoted, "promoted by filter pipeline"); err != nil {
		return err
	}
	_ = f.store.IncrementPeerCounter(authorPubKey, "packets_promo")
	if f.karma != nil {
		_ = f.karma.Reward(authorPubKey, "packet promoted", 1.0)
	}
	return nil
}

// HandleFeedback I-1 quarantine zone — user thumbs_up/thumbs_down.
// thumbs_up → fast-track promote + karma +0.1
// thumbs_down → drop + karma -0.10
func (f *FilterPipeline) HandleFeedback(packetID, feedback string) error {
	if err := f.store.SetShadowFeedback(packetID, feedback); err != nil {
		return err
	}
	pkt, _, err := f.store.GetPacket(packetID)
	if err != nil {
		return err
	}
	switch feedback {
	case "thumbs_up":
		return f.PromotePacket(packetID, pkt.AuthorPubKey)
	case "thumbs_down":
		_ = f.store.UpdatePacketStatus(packetID, FilterStatusDropped, "user thumbs_down")
		if f.karma != nil {
			_ = f.karma.Penalize(pkt.AuthorPubKey, "user thumbs_down", 1.0)
		}
	}
	return nil
}

// --- internal helpers ---

func (f *FilterPipeline) penalize(pubkey []byte, reason string, severity float64) {
	if f.karma != nil {
		_ = f.karma.Penalize(pubkey, reason, severity)
	}
}

func (f *FilterPipeline) persistDropped(p KnowledgePacket, reason, audit string) {
	if err := f.store.InsertPacket(p, FilterStatusDropped); err != nil {
		// Likely duplicate — try update status
		_ = f.store.UpdatePacketStatus(p.ID.String(), FilterStatusDropped, audit)
	}
}

// ---------- L3 RateLimit ----------

// RateLimit token bucket per pubkey.
type RateLimit struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens   float64
	lastFill time.Time
	capacity float64 // 100 tokens
	refill   float64 // per second (100/60s = 1.667/s)
}

// NewRateLimit construct.
func NewRateLimit() *RateLimit {
	return &RateLimit{buckets: make(map[string]*tokenBucket)}
}

// Allow consume 1 token. Return false kalau exceed.
func (r *RateLimit) Allow(pubkey []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := hex.EncodeToString(pubkey)
	b, ok := r.buckets[key]
	if !ok {
		b = &tokenBucket{
			tokens: 100, lastFill: time.Now(),
			capacity: 100, refill: 100.0 / 60.0, // 100 per 60s
		}
		r.buckets[key] = b
	}
	elapsed := time.Since(b.lastFill).Seconds()
	b.tokens = math.Min(b.capacity, b.tokens+elapsed*b.refill)
	b.lastFill = time.Now()
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// ---------- L4 Blacklist ----------

// Blacklist regex-based content filter (PII, secret, prompt-inject).
type Blacklist struct {
	pii            []*regexp.Regexp
	secrets        []*regexp.Regexp
	promptInjects  []*regexp.Regexp
}

// NewBlacklist with default patterns including Indonesia PII (KK, KTP, NPWP,
// BPJS, nomor HP) per AMENDMENTS-V1 Gemini audit.
func NewBlacklist() *Blacklist {
	return &Blacklist{
		pii: []*regexp.Regexp{
			regexp.MustCompile(`\b\d{16}\b`),                            // KK / NIK / credit card
			regexp.MustCompile(`\b\d{15}\b`),                            // NPWP digit-only fallback
			regexp.MustCompile(`\b\d{2}\.\d{3}\.\d{3}\.\d-\d{3}\.\d{3}\b`), // NPWP formatted
			regexp.MustCompile(`\b\d{13}\b`),                            // BPJS
			regexp.MustCompile(`\b08\d{8,11}\b`),                        // HP Indo 08xx
			regexp.MustCompile(`\+62\d{8,12}\b`),                        // HP Indo +62
			regexp.MustCompile(`\b[A-Z]{2}\d{6}[A-Z]\b`),                // passport
			regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),                 // SSN US
		},
		secrets: []*regexp.Regexp{
			regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*\S{20,}`),
			regexp.MustCompile(`(?i)secret[_-]?key\s*[:=]\s*\S{20,}`),
			regexp.MustCompile(`(?i)password\s*[:=]\s*\S{8,}`),
			regexp.MustCompile(`-----BEGIN [A-Z ]+PRIVATE KEY-----`),
			regexp.MustCompile(`(?i)bearer\s+[a-z0-9._-]{20,}`),
			regexp.MustCompile(`(?i)ghp_[a-zA-Z0-9]{36}`),  // GitHub token
			regexp.MustCompile(`(?i)sk-[a-zA-Z0-9]{40,}`),  // OpenAI / similar
		},
		promptInjects: []*regexp.Regexp{
			regexp.MustCompile(`(?i)ignore\s+(previous|all|above)\s+(instruction|prompt|rule)`),
			regexp.MustCompile(`(?i)you\s+are\s+now\s+`),
			regexp.MustCompile(`(?i)reveal\s+(your|the)\s+(system\s+prompt|instruction)`),
			regexp.MustCompile(`(?i)pretend\s+(you|to be)`),
			regexp.MustCompile(`(?i)(act|behave|respond)\s+as\s+(if|though)`),
		},
	}
}

// Scan return (reason, blocked). Blocked=true kalau match any pattern.
func (b *Blacklist) Scan(payload []byte) (string, bool) {
	for _, p := range b.pii {
		if p.Match(payload) {
			return "PII pattern: " + p.String(), true
		}
	}
	for _, p := range b.secrets {
		if p.Match(payload) {
			return "secret pattern", true // ngga reveal regex untuk avoid hint to attacker
		}
	}
	for _, p := range b.promptInjects {
		if p.Match(payload) {
			return "prompt injection pattern", true
		}
	}
	return "", false
}

// ---------- L8 ConsensusGate ----------

// ConsensusGate adaptive consensus check (W-7).
type ConsensusGate struct {
	mu sync.RWMutex
}

// NewConsensusGate construct.
func NewConsensusGate() *ConsensusGate {
	return &ConsensusGate{}
}

// Eligible return true kalau packet shadow eligible promote per consensus.
//
// Adaptive thresholds (W-7):
//   <5 active peers : minPeers=1, minWait=4h
//   <20 active peers: minPeers=2, minWait=12h
//   ≥20 active peers: minPeers=3, minWait=24h
//
// Cosine boost (per filter L7): kalau verdict=Boost (>0.7), threshold turun 1.
func (c *ConsensusGate) Eligible(store *MeshDB, packetID string, cosineVerdict CosineVerdict) bool {
	shadow, err := store.GetShadow(packetID)
	if err != nil {
		return false
	}

	// Adaptive sizing
	peers, _ := store.ListPeers(1000)
	activePeers := 0
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for _, p := range peers {
		if p.LastSeen.After(cutoff) {
			activePeers++
		}
	}
	minPeers, minWait := 3, 24*time.Hour
	switch {
	case activePeers < 5:
		minPeers, minWait = 1, 4*time.Hour
	case activePeers < 20:
		minPeers, minWait = 2, 12*time.Hour
	}
	if cosineVerdict == CosineVerdictBoost && minPeers > 1 {
		minPeers-- // boost lower threshold
	}

	if shadow.ConsensusCount < minPeers {
		return false
	}
	if time.Since(shadow.ShadowSince) < minWait {
		return false
	}
	return true
}
