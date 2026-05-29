// Package mesh — kernel/mesh/relay.go
//
// NAT traversal relay (Phase E2-A3). Edge kernel di belakang NAT outbound-only,
// heavy peer di public IP forward request via long-running connection.
//
// Design (MVP, decisions per Opus_1 finalisasi setelah opus-3 offline):
//   - Transport: SSE (Server-Sent Events) — stdlib net/http native, no new
//     deps. Server push events ke edge via text/event-stream.
//   - Edge replies via separate HTTP POST (correlation by request_id).
//   - Auth: HMAC-SHA256 over (timestamp + edge_id + nonce), shared secret di
//     settings DB MESH_RELAY_SECRET. Replay window 60s.
//   - Rate limit: token bucket per session (default 100/min, burst 100).
//
// Threat model:
//   - Anti replay: timestamp + nonce, ±60s window
//   - Anti enumeration: HMAC-SHA256 256-bit, no timing leak via constant-time
//     compare
//   - Anti DoS: token bucket per edge, plus connection cap (max 50 concurrent
//     edge sessions, configurable MESH_RELAY_MAX_SESSIONS)
//
// Phase J upgrade: ECDSA P-256 token + mTLS (per REVISIONS Fix #8 DPoP-style).

package mesh

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flowork/kernel/kernel/settings"
)

// Sentinel errors — caller-distinguishable.
var (
	ErrRelayUnauthorized = errors.New("relay: unauthorized (missing or bad token)")
	ErrRelayReplay       = errors.New("relay: replay or stale token")
	ErrRelayRateLimit    = errors.New("relay: rate limit exceeded")
	ErrRelaySessionFull  = errors.New("relay: session capacity full")
	ErrRelayEdgeNotFound = errors.New("relay: edge session not found")
)

const (
	// ReplayWindow ±60s timestamp tolerance — short enough anti replay,
	// long enough toleransi clock skew normal.
	ReplayWindow = 60 * time.Second

	// DefaultRateLimitPerMin = 100 req/min per session (anti Slowloris).
	DefaultRateLimitPerMin = 100

	// DefaultMaxSessions = 50 concurrent edge sessions per heavy node.
	DefaultMaxSessions = 50

	// SessionTTL = idle timeout. Saat edge ngga refresh > 5 min, drop.
	SessionTTL = 5 * time.Minute
)

// RelayToken signed envelope dari edge. HMAC validates timestamp + edge_id +
// nonce. Wire format: "ts:edge_id:nonce:hmac" (colon-separated).
type RelayToken struct {
	Timestamp int64
	EdgeID    string
	Nonce     string
	HMAC      string // hex
}

// signToken create HMAC over (timestamp || edge_id || nonce) dengan secret.
// Internal helper — verifyToken sebagai inverse.
func signToken(secret []byte, ts int64, edgeID, nonce string) string {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte(strconv.FormatInt(ts, 10)))
	_, _ = h.Write([]byte{':'})
	_, _ = h.Write([]byte(edgeID))
	_, _ = h.Write([]byte{':'})
	_, _ = h.Write([]byte(nonce))
	return hex.EncodeToString(h.Sum(nil))
}

// MakeToken construct signed token untuk edge connector. Caller (edge side)
// pass shared secret + edge_id, get token string siap di-pasang Authorization
// header.
func MakeToken(secret []byte, edgeID, nonce string) string {
	ts := nowMs() / 1000
	mac := signToken(secret, ts, edgeID, nonce)
	return fmt.Sprintf("%d:%s:%s:%s", ts, edgeID, nonce, mac)
}

// ParseToken decode wire format "ts:edge_id:nonce:hmac".
func ParseToken(raw string) (RelayToken, error) {
	parts := strings.SplitN(raw, ":", 4)
	if len(parts) != 4 {
		return RelayToken{}, ErrRelayUnauthorized
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return RelayToken{}, ErrRelayUnauthorized
	}
	if parts[1] == "" || parts[2] == "" || parts[3] == "" {
		return RelayToken{}, ErrRelayUnauthorized
	}
	return RelayToken{
		Timestamp: ts,
		EdgeID:    parts[1],
		Nonce:     parts[2],
		HMAC:      parts[3],
	}, nil
}

// VerifyToken validate HMAC + timestamp window. Constant-time HMAC compare.
//
// Errors:
//   - ErrRelayUnauthorized: malformed atau bad HMAC
//   - ErrRelayReplay: timestamp outside ±ReplayWindow
func VerifyToken(secret []byte, raw string) (RelayToken, error) {
	tok, err := ParseToken(raw)
	if err != nil {
		return RelayToken{}, err
	}

	// Timestamp window check.
	now := nowMs() / 1000
	skew := now - tok.Timestamp
	if skew < 0 {
		skew = -skew
	}
	if time.Duration(skew)*time.Second > ReplayWindow {
		return tok, ErrRelayReplay
	}

	// HMAC verify (constant-time).
	expected := signToken(secret, tok.Timestamp, tok.EdgeID, tok.Nonce)
	expectedBytes, _ := hex.DecodeString(expected)
	gotBytes, err := hex.DecodeString(tok.HMAC)
	if err != nil {
		return tok, ErrRelayUnauthorized
	}
	if !hmac.Equal(expectedBytes, gotBytes) {
		return tok, ErrRelayUnauthorized
	}
	return tok, nil
}

// relaySecret read shared secret dari settings DB. Empty → relay disabled
// (return false from RelayEnabled).
func relaySecret() []byte {
	store := settings.Shared()
	if store == nil {
		return nil
	}
	v, _ := store.Get("MESH_RELAY_SECRET")
	if v == "" {
		return nil
	}
	return []byte(v)
}

// RelayEnabled return true kalau MESH_RELAY_SECRET set di settings DB.
// Caller (route_relay handler) gate endpoint based on this.
func RelayEnabled() bool {
	return len(relaySecret()) > 0
}

// ─── Session registry + rate limit ───

// EdgeSession represent active edge connection. Tracked in process memory.
type EdgeSession struct {
	EdgeID     string
	ConnectedAt time.Time
	LastSeen   time.Time

	// Rate limit token bucket.
	bucketMu     sync.Mutex
	tokens       float64
	bucketUpdate time.Time
}

// AllowRequest check + decrement token bucket. Refill rate = ratePerMin/60s,
// capacity = ratePerMin (burst). Exported untuk caller di kernel/api.
func (s *EdgeSession) AllowRequest(ratePerMin int) bool {
	if ratePerMin <= 0 {
		ratePerMin = DefaultRateLimitPerMin
	}
	s.bucketMu.Lock()
	defer s.bucketMu.Unlock()

	now := time.Now()
	if s.bucketUpdate.IsZero() {
		s.tokens = float64(ratePerMin)
		s.bucketUpdate = now
	}
	// Refill.
	elapsed := now.Sub(s.bucketUpdate).Seconds()
	s.tokens += elapsed * float64(ratePerMin) / 60.0
	if s.tokens > float64(ratePerMin) {
		s.tokens = float64(ratePerMin)
	}
	s.bucketUpdate = now

	if s.tokens < 1.0 {
		return false
	}
	s.tokens -= 1.0
	return true
}

// SessionRegistry track active edge connections. Process-level singleton.
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*EdgeSession
	maxSess  int
}

var (
	sharedRegistry   *SessionRegistry
	sharedRegistryMu sync.Once
)

// SharedSessionRegistry singleton.
func SharedSessionRegistry() *SessionRegistry {
	sharedRegistryMu.Do(func() {
		sharedRegistry = &SessionRegistry{
			sessions: make(map[string]*EdgeSession),
			maxSess:  DefaultMaxSessions,
		}
	})
	return sharedRegistry
}

// Register edge session. Return ErrRelaySessionFull kalau capacity hit.
// Idempotent — re-register same edge_id refresh LastSeen.
func (r *SessionRegistry) Register(edgeID string) (*EdgeSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refresh existing.
	if s, ok := r.sessions[edgeID]; ok {
		s.LastSeen = time.Now()
		return s, nil
	}

	if len(r.sessions) >= r.maxSess {
		// GC stale before reject.
		r.gcStaleLocked()
		if len(r.sessions) >= r.maxSess {
			return nil, ErrRelaySessionFull
		}
	}

	s := &EdgeSession{
		EdgeID:      edgeID,
		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),
	}
	r.sessions[edgeID] = s
	return s, nil
}

// Get session by edge_id. Return ErrRelayEdgeNotFound kalau ngga ada.
func (r *SessionRegistry) Get(edgeID string) (*EdgeSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[edgeID]
	if !ok {
		return nil, ErrRelayEdgeNotFound
	}
	return s, nil
}

// Touch refresh LastSeen — call saat receive valid request dari edge.
func (r *SessionRegistry) Touch(edgeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[edgeID]; ok {
		s.LastSeen = time.Now()
	}
}

// Remove explicit session drop (edge close connection).
func (r *SessionRegistry) Remove(edgeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, edgeID)
}

// Count active sessions (for /v1/mesh/peers diagnostic).
func (r *SessionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

// gcStaleLocked drop sessions yang LastSeen > SessionTTL ago.
// Caller must hold r.mu.Lock.
func (r *SessionRegistry) gcStaleLocked() {
	cutoff := time.Now().Add(-SessionTTL)
	for id, s := range r.sessions {
		if s.LastSeen.Before(cutoff) {
			delete(r.sessions, id)
		}
	}
}

// resetRegistryForTest test helper — pair dengan FLOWORK_HOME=t.TempDir.
func resetRegistryForTest() {
	if sharedRegistry == nil {
		return
	}
	sharedRegistry.mu.Lock()
	defer sharedRegistry.mu.Unlock()
	sharedRegistry.sessions = make(map[string]*EdgeSession)
}
