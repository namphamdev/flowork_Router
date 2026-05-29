// Transport: TCP + NaCl box (curve25519 + xsalsa20 + poly1305) authenticated
// encryption between mesh peers. Each frame is a length-prefixed ciphertext
// with a 24-byte random nonce prepended.
//
// Wire format (per frame):
//
//	[4 bytes BE length][24 byte nonce][ciphertext]
//
// where ciphertext = box.Seal(plaintext, nonce, peerPub, ourPriv).
package mesh

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/nacl/box"
)

const (
	// maxFrameSize caps a single encrypted message at 1 MiB. Larger payloads
	// must be chunked by the caller — prevents a malicious peer from
	// forcing unbounded memory allocation.
	maxFrameSize = 4 << 20

	handshakeTimeout = 10 * time.Second
	frameReadTimeout = 60 * time.Second
)

// KeyPair holds an X25519 keypair used by box.Seal / box.Open.
type KeyPair struct {
	Public  *[32]byte
	Private *[32]byte
}

// GenerateKeyPair creates a fresh X25519 keypair.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("nacl keygen: %w", err)
	}
	return &KeyPair{Public: pub, Private: priv}, nil
}

// PublicHex returns the hex-encoded public key for advertising in mDNS TXT.
func (k *KeyPair) PublicHex() string {
	return hex.EncodeToString(k.Public[:])
}

// DecodePublicKey parses a hex string produced by PublicHex into a [32]byte.
func DecodePublicKey(s string) (*[32]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode pubkey: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("pubkey wrong length: got %d want 32", len(b))
	}
	var out [32]byte
	copy(out[:], b)
	return &out, nil
}

// Session is an active encrypted connection to one peer.
type Session struct {
	conn    net.Conn
	ourPriv *[32]byte
	peerPub *[32]byte

	wmu sync.Mutex

	// Replay defense (Gemini audit fix Bug 7.2): bounded set of recently
	// accepted nonces. Since nonces are 24-byte random, collision on
	// legitimate traffic is negligible — any repeat indicates an attacker
	// replaying a captured ciphertext. Bounded at replayNonceWindow to
	// cap memory (LRU-style eviction: oldest insert drops first).
	nonceMu    sync.Mutex
	seenNonces map[[24]byte]struct{}
	nonceOrder [][24]byte
}

// replayNonceWindow is how many recent nonces we retain per session for
// replay detection. 4096 × 24B ≈ 96KB per session, cheap.
const replayNonceWindow = 4096

// Dial opens a TCP connection to peer and negotiates an encrypted session.
// The caller supplies our keypair and the peer's published public key.
// Dial is blocking — run it in a goroutine if the caller has other work.
func Dial(ctx context.Context, addr string, our *KeyPair, peerPub *[32]byte) (*Session, error) {
	var d net.Dialer
	d.Timeout = handshakeTimeout
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return &Session{conn: conn, ourPriv: our.Private, peerPub: peerPub}, nil
}

// Send encrypts and writes one message to the peer.
func (s *Session) Send(msg []byte) error {
	if len(msg) > maxFrameSize-box.Overhead-24 {
		return fmt.Errorf("message too large (%d bytes, max %d)", len(msg), maxFrameSize-box.Overhead-24)
	}

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("nonce: %w", err)
	}

	ciphertext := box.Seal(nil, msg, &nonce, s.peerPub, s.ourPriv)

	frame := make([]byte, 4+24+len(ciphertext))
	binary.BigEndian.PutUint32(frame[:4], uint32(24+len(ciphertext)))
	copy(frame[4:28], nonce[:])
	copy(frame[28:], ciphertext)

	s.wmu.Lock()
	defer s.wmu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(frameReadTimeout))
	if _, err := s.conn.Write(frame); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}

// Recv reads and decrypts the next message from the peer.
// Returns io.EOF cleanly when the peer closes the connection.
func (s *Session) Recv() ([]byte, error) {
	_ = s.conn.SetReadDeadline(time.Now().Add(frameReadTimeout))

	var lenBuf [4]byte
	if _, err := io.ReadFull(s.conn, lenBuf[:]); err != nil {
		return nil, err
	}
	total := binary.BigEndian.Uint32(lenBuf[:])
	if total > maxFrameSize {
		return nil, fmt.Errorf("frame too large: %d", total)
	}
	if total < 24+box.Overhead {
		return nil, fmt.Errorf("frame too small: %d", total)
	}

	buf := make([]byte, total)
	if _, err := io.ReadFull(s.conn, buf); err != nil {
		return nil, fmt.Errorf("read frame body: %w", err)
	}

	var nonce [24]byte
	copy(nonce[:], buf[:24])

	// Gemini audit fix Bug 7.2: refuse nonces we've already accepted on
	// this session. Prevents replay of captured ciphertext.
	if s.nonceSeenAndRecord(nonce) {
		return nil, errors.New("replay detected: nonce already used on this session")
	}

	plaintext, ok := box.Open(nil, buf[24:], &nonce, s.peerPub, s.ourPriv)
	if !ok {
		return nil, errors.New("decrypt failed: invalid signature or tampered ciphertext")
	}
	return plaintext, nil
}

// nonceSeenAndRecord returns true if we've already accepted this nonce on
// the current session; otherwise records it. LRU-style eviction when the
// window fills up.
func (s *Session) nonceSeenAndRecord(n [24]byte) bool {
	s.nonceMu.Lock()
	defer s.nonceMu.Unlock()
	if s.seenNonces == nil {
		s.seenNonces = make(map[[24]byte]struct{}, replayNonceWindow)
	}
	if _, ok := s.seenNonces[n]; ok {
		return true
	}
	s.seenNonces[n] = struct{}{}
	s.nonceOrder = append(s.nonceOrder, n)
	if len(s.nonceOrder) > replayNonceWindow {
		evict := s.nonceOrder[0]
		s.nonceOrder = s.nonceOrder[1:]
		delete(s.seenNonces, evict)
	}
	return false
}

// Close releases the underlying TCP connection.
func (s *Session) Close() error {
	return s.conn.Close()
}

// Handler processes one inbound message and optionally returns a reply.
// Return nil reply to stay silent.
type Handler func(peerPub *[32]byte, msg []byte) (reply []byte, err error)

// Listener accepts inbound mesh connections and dispatches to a handler.
type Listener struct {
	ln   net.Listener
	keys *KeyPair
	h    Handler

	// allowedPeers: if non-nil, only connections from peers whose pubkey is
	// in this set are accepted. Populated by the discovery layer.
	allowedPeers   map[[32]byte]struct{}
	allowedPeersMu sync.RWMutex

	// Audit GAP #9 — track active per-connection handlers so Serve()'s ctx
	// cancel path can wait for them to finish instead of leaving them blocked
	// on Recv while the listener is already closed. Prevents goroutine leak
	// accumulation across reconnect cycles.
	activeHandlers sync.WaitGroup

	// EXTBUG-007: inbound-connection rate limiter. Before this, any peer
	// (allowed or not) could open thousands of sockets — each consumed a
	// goroutine + memory for handshakeTimeout seconds before rejection.
	// connBudget tracks per-remote-IP attempts within a sliding window so
	// we can drop suspicious bursts before spawning a handler.
	connBudgetMu sync.Mutex
	connBudget   map[string][]time.Time
}

// maxConnPerMinutePerIP caps inbound handshake attempts per remote IP per
// minute. 20 is generous for legitimate reconnect churn and tight enough
// that a single attacker cannot pin a handler goroutine for every second
// of the handshake timeout. EXTBUG-007.
const maxConnPerMinutePerIP = 20

// NewListener binds a TCP listener on 127.0.0.1:port (or 0.0.0.0:port if
// bindAll=true) for mesh peers. Handler is invoked for each received message.
func NewListener(port int, bindAll bool, keys *KeyPair, h Handler) (*Listener, error) {
	host := "127.0.0.1"
	if bindAll {
		host = "0.0.0.0" // LAN mesh — peers are on other hosts
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("mesh listen: %w", err)
	}
	return &Listener{
		ln:           ln,
		keys:         keys,
		h:            h,
		allowedPeers: make(map[[32]byte]struct{}),
		connBudget:   make(map[string][]time.Time),
	}, nil
}

// acceptWithinBudget returns true if the remote at addr should be allowed
// to proceed with a handshake. Side effect: records the current attempt.
// EXTBUG-007.
func (l *Listener) acceptWithinBudget(addr net.Addr) bool {
	host := ""
	if ta, ok := addr.(*net.TCPAddr); ok {
		host = ta.IP.String()
	} else if addr != nil {
		host = addr.String()
	}
	if host == "" {
		return true // unknown — don't false-positive block
	}
	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)
	l.connBudgetMu.Lock()
	defer l.connBudgetMu.Unlock()
	// Evict samples older than the window; retain most recent tail.
	prev := l.connBudget[host]
	kept := prev[:0]
	for _, t := range prev {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= maxConnPerMinutePerIP {
		l.connBudget[host] = kept
		return false
	}
	l.connBudget[host] = append(kept, now)
	return true
}

// Addr returns the bound network address (useful when port=0 was passed).
func (l *Listener) Addr() net.Addr {
	return l.ln.Addr()
}

// Port returns the concrete TCP port bound (useful when port=0 was passed).
func (l *Listener) Port() int {
	if tcp, ok := l.ln.Addr().(*net.TCPAddr); ok {
		return tcp.Port
	}
	return 0
}

// AllowPeer registers a peer's pubkey as authorized to connect.
// Without AllowPeer, all connections are rejected after handshake.
func (l *Listener) AllowPeer(pub *[32]byte) {
	l.allowedPeersMu.Lock()
	defer l.allowedPeersMu.Unlock()
	l.allowedPeers[*pub] = struct{}{}
}

// Serve accepts connections until ctx is cancelled or the listener closes.
// Waits for all in-flight handlers (up to handshakeTimeout) before returning
// so goroutines are not left stranded on Recv after Close.
func (l *Listener) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = l.ln.Close()
	}()

	for {
		conn, err := l.ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				// Audit GAP #9: bound the wait so a pathological handler
				// can't block shutdown forever. Individual handlers honor
				// SetReadDeadline(handshakeTimeout).
				done := make(chan struct{})
				go func() {
					l.activeHandlers.Wait()
					close(done)
				}()
				select {
				case <-done:
				case <-time.After(2 * handshakeTimeout):
				}
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		// EXTBUG-007: rate-limit bursts before spawning a handler.
		if !l.acceptWithinBudget(conn.RemoteAddr()) {
			_ = conn.Close()
			continue
		}
		l.activeHandlers.Add(1)
		go func(c net.Conn) {
			defer l.activeHandlers.Done()
			l.handleConn(ctx, c)
		}(conn)
	}
}

// Contains implements AllowedPeers — snapshot lookup into the allowed set.
// Used by transport Handshake implementations (V1/V2) to gate peer identity.
func (l *Listener) Contains(pub [32]byte) bool {
	l.allowedPeersMu.RLock()
	defer l.allowedPeersMu.RUnlock()
	_, ok := l.allowedPeers[pub]
	return ok
}

// bufferedPeerConn wraps a net.Conn with a *bufio.Reader so dispatchHandshake
// can peek the first byte without consuming it. Satisfies PeerConn + keeps
// the underlying net.Conn accessible for SetDeadline / Close.
type bufferedPeerConn struct {
	net.Conn
	br *bufio.Reader
}

func (b *bufferedPeerConn) Read(p []byte) (int, error) { return b.br.Read(p) }

// dispatchHandshake routes an incoming conn to the correct Transport by
// peeking the first byte. v2 handshakes start with 0x02 version tag; v1
// legacy sends a raw 32-byte pubkey (first byte = pubkey[0], arbitrary).
//
// rc149-opus2 A04 wire — previously Listener.handleConn hardcoded V1 flow
// and bypassed the registered Transport. Now dispatches via registry so
// V2 challenge-response (Gemini Bug 7.1 mitigation) is live on the wire.
//
// Override: FLOWORK_MESH_PROTO=v1 forces V1-only (legacy compat), =v2
// rejects V1 attempts. Empty (default) = auto-detect.
func (l *Listener) dispatchHandshake(ctx context.Context, conn net.Conn) (*Session, error) {
	br := bufio.NewReader(conn)
	first, err := br.Peek(1)
	if err != nil {
		return nil, fmt.Errorf("peek: %w", err)
	}

	override := os.Getenv("FLOWORK_MESH_PROTO")
	var t Transport
	switch {
	case override == "v1":
		t = GetTransport("v1")
	case override == "v2":
		t = GetTransport("v2")
		if first[0] != v2Version {
			return nil, fmt.Errorf("mesh v2 enforced, got legacy v1 handshake from %s", conn.RemoteAddr())
		}
	case first[0] == v2Version:
		t = GetTransport("v2")
	default:
		t = GetTransport("v1")
	}
	if t == nil {
		return nil, fmt.Errorf("transport dispatch: no matching registered transport (override=%q first-byte=0x%02x)", override, first[0])
	}

	wrapped := &bufferedPeerConn{Conn: conn, br: br}
	return t.Handshake(ctx, wrapped, l.keys, l)
}

// handleConn runs one incoming peer's message loop.
// rc149-opus2 A04 wire — dispatch to Transport.Handshake via registry
// instead of hardcoded V1 flow. Supports V1 (legacy plaintext pubkey) +
// V2 (challenge-response with nonce replay defense) via peek dispatcher.
func (l *Listener) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	sess, err := l.dispatchHandshake(ctx, conn)
	if err != nil {
		return // handshake failed — reject silently (avoid info leak to probes)
	}

	// Clear deadline post-handshake so long-lived sessions aren't torn down
	// mid-message. sess.Recv/Send manage per-op deadlines internally.
	_ = conn.SetReadDeadline(time.Time{})

	peerPub := sess.peerPub
	for {
		if ctx.Err() != nil {
			return
		}
		msg, err := sess.Recv()
		if err != nil {
			return
		}
		reply, err := l.h(peerPub, msg)
		if err != nil || reply == nil {
			continue
		}
		if err := sess.Send(reply); err != nil {
			return
		}
	}
}

// Close stops accepting new connections.
func (l *Listener) Close() error {
	return l.ln.Close()
}

// DialAuth opens a session AND sends our pubkey first (matching
// Listener.handleConn's handshake expectation).
func DialAuth(ctx context.Context, addr string, our *KeyPair, peerPub *[32]byte) (*Session, error) {
	s, err := Dial(ctx, addr, our, peerPub)
	if err != nil {
		return nil, err
	}
	_ = s.conn.SetWriteDeadline(time.Now().Add(handshakeTimeout))
	if _, err := s.conn.Write(our.Public[:]); err != nil {
		s.Close()
		return nil, fmt.Errorf("send pubkey: %w", err)
	}
	return s, nil
}
