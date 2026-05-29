// Package mesh — gossip_signed.go: M03 Signed Gossip Envelope.
//
// Per ROADMAP_AKTIF Tier 4 M03: existing CRDT push (`crdt_push.go`) basic.
// Belum: signed envelope wrapper dengan Ed25519 verify per packet, sehingga
// peer dengan stolen IP tetep ditolak kalau ngga punya private key.
//
// Envelope structure:
//
//	┌─────────────────────────────────────────┐
//	│ SignedEnvelope                          │
//	│   ├─ payload (bytes)         — actual gossip data (CRDT delta, drawer chunk, etc)
//	│   ├─ sender_pub_key (32B)    — Ed25519 public key sender
//	│   ├─ timestamp (int64)       — Unix nanos, anti-replay window check
//	│   ├─ nonce (16B)             — random, dedup id
//	│   └─ signature (64B)         — Ed25519 sign(payload || pub_key || timestamp || nonce)
//	└─────────────────────────────────────────┘
//
// Anti-replay: timestamp must be within ±5 min window (clock skew tolerance).
// Anti-DoS: nonce hash recorded, repeat reject.

package mesh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

// EnvelopeReplayWindow — max clock skew tolerated antar peer (5 menit).
const EnvelopeReplayWindow = 5 * time.Minute

// SignedEnvelope — wire format untuk gossip dengan crypto verification.
type SignedEnvelope struct {
	Payload    []byte `json:"payload"`
	SenderPub  []byte `json:"sender_pub"` // 32 bytes Ed25519 public
	Timestamp  int64  `json:"timestamp"`  // Unix nanos
	Nonce      []byte `json:"nonce"`      // 16 random bytes
	Signature  []byte `json:"signature"`  // 64 bytes Ed25519
}

// nonceTracker — anti-replay nonce ledger (in-memory, prune old).
type nonceTracker struct {
	mu    sync.Mutex
	seen  map[string]time.Time
	maxAge time.Duration
}

var defaultNonceTracker = &nonceTracker{
	seen:   map[string]time.Time{},
	maxAge: EnvelopeReplayWindow * 2,
}

// SignEnvelope — wrap payload dalam signed envelope.
//
// Caller pass private key (32 bytes seed atau 64 bytes ed25519.PrivateKey).
// Returns envelope ready untuk transmit ke peer.
func SignEnvelope(payload []byte, privKey ed25519.PrivateKey) (*SignedEnvelope, error) {
	if len(privKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("gossip_signed: private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(privKey))
	}
	if len(payload) == 0 {
		return nil, errors.New("gossip_signed: payload empty")
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("gossip_signed: nonce gen: %w", err)
	}

	pub := privKey.Public().(ed25519.PublicKey)
	timestamp := time.Now().UnixNano()

	signedBytes := buildSignedBytes(payload, pub, timestamp, nonce)
	sig := ed25519.Sign(privKey, signedBytes)

	return &SignedEnvelope{
		Payload:   payload,
		SenderPub: pub,
		Timestamp: timestamp,
		Nonce:     nonce,
		Signature: sig,
	}, nil
}

// VerifyEnvelope — verify signature + timestamp window + nonce uniqueness.
//
// Returns:
//   - nil error: valid, payload safe to process
//   - error: rejected (signature fail, replay window, duplicate nonce)
//
// Caller WAJIB additional check: SenderPub matches peer karma whitelist /
// blocklist (separate layer).
func VerifyEnvelope(env *SignedEnvelope) error {
	if env == nil {
		return errors.New("gossip_signed: nil envelope")
	}
	if len(env.SenderPub) != ed25519.PublicKeySize {
		return fmt.Errorf("gossip_signed: pub key wrong size: %d", len(env.SenderPub))
	}
	if len(env.Signature) != ed25519.SignatureSize {
		return fmt.Errorf("gossip_signed: signature wrong size: %d", len(env.Signature))
	}
	if len(env.Nonce) != 16 {
		return fmt.Errorf("gossip_signed: nonce wrong size: %d", len(env.Nonce))
	}

	// Anti-replay: timestamp window check
	now := time.Now().UnixNano()
	skew := time.Duration(now - env.Timestamp)
	if skew < -EnvelopeReplayWindow || skew > EnvelopeReplayWindow {
		return fmt.Errorf("gossip_signed: timestamp out of window (skew=%v)", skew)
	}

	// Verify signature
	signedBytes := buildSignedBytes(env.Payload, env.SenderPub, env.Timestamp, env.Nonce)
	if !ed25519.Verify(env.SenderPub, signedBytes, env.Signature) {
		return errors.New("gossip_signed: signature verification failed")
	}

	// Anti-replay: nonce uniqueness
	nonceKey := string(env.Nonce)
	if !defaultNonceTracker.markSeen(nonceKey) {
		return errors.New("gossip_signed: duplicate nonce (replay attack?)")
	}

	return nil
}

// markSeen — true kalau nonce baru, false kalau duplicate.
func (n *nonceTracker) markSeen(key string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.seen[key]; exists {
		return false
	}
	n.seen[key] = time.Now()

	// Prune old entries (lazy — every 100 inserts ke nonceTracker)
	if len(n.seen) > 100 && len(n.seen)%100 == 0 {
		cutoff := time.Now().Add(-n.maxAge)
		for k, t := range n.seen {
			if t.Before(cutoff) {
				delete(n.seen, k)
			}
		}
	}
	return true
}

// buildSignedBytes — canonical concatenation: payload || pub || ts || nonce.
func buildSignedBytes(payload, pub []byte, timestamp int64, nonce []byte) []byte {
	out := make([]byte, 0, len(payload)+len(pub)+8+len(nonce))
	out = append(out, payload...)
	out = append(out, pub...)
	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, uint64(timestamp))
	out = append(out, tsBytes...)
	out = append(out, nonce...)
	return out
}
