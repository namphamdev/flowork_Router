// Relay server — a tiny TCP rendezvous box that forwards frames between
// FLOWORK mesh peers that can't reach each other directly (different LAN,
// behind NAT, etc.). The relay is intentionally dumb: it never decrypts,
// inspects, or stores payloads — it just routes.
//
// # Wire format (peer ↔ relay)
//
// Each frame on the relay link is:
//
//	[1 byte  msg_type]
//	[4 bytes BE body_len]
//	[body...]
//
// Message types:
//
//	0x01 REGISTER     body = 32-byte pubkey || optional UTF-8 label
//	0x02 FORWARD      body = 32-byte to_pubkey || opaque NaCl-sealed payload
//	0x03 HEARTBEAT    body = empty
//	0x04 INCOMING     body = 32-byte from_pubkey || opaque payload  (relay → peer)
//	0x05 ACK          body = empty (reserved; not currently emitted)
//	0x06 ERROR        body = UTF-8 error message
//	0x07 CHALLENGE    body = 32-byte random nonce (relay → peer, post-REGISTER) [BUG-W41]
//	0x08 SIGNATURE    body = 64-byte ed25519 signature(nonce) (peer → relay) [BUG-W41]
//
// Sprint 3.5e (BUG-W40 + BUG-W41 fix): challenge-response handshake.
// Setelah REGISTER, server kirim CHALLENGE dengan random nonce. Client harus
// reply SIGNATURE = ed25519.Sign(privkey, nonce). Server verify with claimed
// pubkey. Kalau invalid → close. Tidak ada lagi pubkey spoofing — client
// wajib bukti owns privkey via cryptographic signature.
//
// The opaque payload is whatever `transport.go` produced — typically a full
// peer-to-peer frame `[len][nonce][ciphertext]`. The relay never decrypts.
//
// # Limits
//
// Frames are capped at 1 MiB (matches transport.go). Peers idle longer than
// 90 seconds without a heartbeat are evicted; their pubkey becomes free for
// re-registration.
package mesh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	relayMsgRegister  byte = 0x01
	relayMsgForward   byte = 0x02
	relayMsgHeartbeat byte = 0x03
	relayMsgIncoming  byte = 0x04
	relayMsgError     byte = 0x06
	// Sprint 3.5e (BUG-W41): challenge-response handshake messages.
	relayMsgChallenge byte = 0x07
	relayMsgSignature byte = 0x08

	relayMaxFrame   = 4 << 20 // 4 MiB
	relayIdleTTL    = 30 * time.Second // Sprint 3.5d (BUG-W12 fix): FQP-6 hot-reload spec ≤30s
	relayReadDeadln = 120 * time.Second
)

// ─── Server ─────────────────────────────────────────────────────────

// RelayServer accepts peer connections and routes FORWARD frames by pubkey.
type RelayServer struct {
	ln net.Listener

	mu    sync.RWMutex
	peers map[[32]byte]*relayPeer // keyed by pubkey
}

type relayPeer struct {
	pubkey   [32]byte
	label    string
	conn     net.Conn
	lastSeen time.Time

	// writeMu guards conn.Write — multiple goroutines may forward frames
	// to this peer concurrently.
	writeMu sync.Mutex
}

// NewRelayServer binds a TCP listener on the given address (e.g. ":8888"
// or "0.0.0.0:8888"). Pass ":0" to let the OS pick a port; query Addr().
func NewRelayServer(addr string) (*RelayServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("relay listen: %w", err)
	}
	return &RelayServer{
		ln:    ln,
		peers: make(map[[32]byte]*relayPeer),
	}, nil
}

// Addr returns the bound address (useful when port=":0" was passed).
func (s *RelayServer) Addr() net.Addr { return s.ln.Addr() }

// PeerCount returns how many peers are currently registered.
func (s *RelayServer) PeerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.peers)
}

// Peers returns a snapshot of registered peers' pubkey hex + label, useful
// for status dumps.
func (s *RelayServer) Peers() []RelayPeerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RelayPeerInfo, 0, len(s.peers))
	for _, p := range s.peers {
		out = append(out, RelayPeerInfo{
			PubKey:   p.pubkey,
			Label:    p.label,
			LastSeen: p.lastSeen,
		})
	}
	return out
}

// RelayPeerInfo is the public-readable view of a registered peer.
type RelayPeerInfo struct {
	PubKey   [32]byte
	Label    string
	LastSeen time.Time
}

// Serve accepts connections until ctx is cancelled or the listener closes.
// Also runs an eviction goroutine for stale peers.
func (s *RelayServer) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = s.ln.Close()
	}()
	go s.evictionLoop(ctx)

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("relay accept: %w", err)
		}
		go s.handleConn(ctx, conn)
	}
}

// handleConn manages one peer's lifetime: handshake (REGISTER), then loop
// reading frames and routing them.
func (s *RelayServer) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	// First frame must be REGISTER.
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	mt, body, err := readRelayFrame(conn)
	if err != nil {
		return
	}
	if mt != relayMsgRegister || len(body) < 32 {
		_ = writeRelayFrame(conn, relayMsgError, []byte("first frame must be REGISTER with 32-byte pubkey"))
		return
	}

	var pub [32]byte
	copy(pub[:], body[:32])
	label := ""
	if len(body) > 32 {
		label = string(body[32:])
	}

	// BUG-W40 + BUG-W41 fix Sprint 3.5e: challenge-response. Server emit
	// random nonce, client harus reply ed25519 signature(nonce) using privkey
	// matching claimed pubkey. Kalau invalid → close. Eliminate pubkey
	// spoofing + add auth gate (only owner of privkey can register).
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		_ = writeRelayFrame(conn, relayMsgError, []byte("server nonce gen failed"))
		return
	}
	if err := writeRelayFrame(conn, relayMsgChallenge, nonce); err != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	mt2, sig, err := readRelayFrame(conn)
	if err != nil {
		return
	}
	if mt2 != relayMsgSignature || len(sig) != ed25519.SignatureSize {
		_ = writeRelayFrame(conn, relayMsgError, []byte("expected SIGNATURE frame (64 bytes)"))
		return
	}
	if !ed25519.Verify(ed25519.PublicKey(pub[:]), nonce, sig) {
		_ = writeRelayFrame(conn, relayMsgError, []byte("signature verification failed (pubkey spoof rejected)"))
		return
	}

	peer := &relayPeer{
		pubkey:   pub,
		label:    label,
		conn:     conn,
		lastSeen: time.Now(),
	}

	// Register (replacing any prior connection for this pubkey).
	var oldConn net.Conn
	s.mu.Lock()
	if old, exists := s.peers[pub]; exists {
		oldConn = old.conn
	}
	s.peers[pub] = peer
	s.mu.Unlock()

	if oldConn != nil {
		_ = oldConn.Close()
	}

	defer func() {
		s.mu.Lock()
		// Only delete if we're still the registered peer (a re-registration
		// from elsewhere may have already overwritten us).
		if cur, ok := s.peers[pub]; ok && cur == peer {
			delete(s.peers, pub)
		}
		s.mu.Unlock()
	}()

	for {
		if ctx.Err() != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(relayReadDeadln))
		mt, body, err := readRelayFrame(conn)
		if err != nil {
			return
		}
		s.mu.Lock()
		peer.lastSeen = time.Now()
		s.mu.Unlock()

		switch mt {
		case relayMsgHeartbeat:
			// already touched lastSeen above
		case relayMsgForward:
			if len(body) < 32 {
				_ = peer.send(relayMsgError, []byte("FORWARD body too short"))
				continue
			}
			var to [32]byte
			copy(to[:], body[:32])
			payload := body[32:]
			s.mu.RLock()
			dst, ok := s.peers[to]
			s.mu.RUnlock()
			if !ok {
				_ = peer.send(relayMsgError, []byte("no such peer"))
				continue
			}
			// INCOMING body = from_pubkey || payload (so receiver knows sender).
			out := make([]byte, 32+len(payload))
			copy(out[:32], pub[:])
			copy(out[32:], payload)
			if err := dst.send(relayMsgIncoming, out); err != nil {
				_ = peer.send(relayMsgError, []byte("delivery failed: "+err.Error()))
			}
		case relayMsgRegister:
			// Late re-register — just update label.
			if len(body) > 32 {
				peer.label = string(body[32:])
			}
		default:
			_ = peer.send(relayMsgError, []byte(fmt.Sprintf("unknown msg_type 0x%02x", mt)))
		}
	}
}

// evictionLoop drops peers that haven't sent any frame within relayIdleTTL.
func (s *RelayServer) evictionLoop(ctx context.Context) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			cutoff := time.Now().Add(-relayIdleTTL)
			connsToClose := []net.Conn{}
			s.mu.Lock()
			for k, p := range s.peers {
				if p.lastSeen.Before(cutoff) {
					connsToClose = append(connsToClose, p.conn)
					delete(s.peers, k)
				}
			}
			s.mu.Unlock()

			for _, conn := range connsToClose {
				_ = conn.Close()
			}
		}
	}
}

// send writes a relay frame to this peer with locking so concurrent
// forwarders don't interleave bytes.
func (p *relayPeer) send(mt byte, body []byte) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_ = p.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	return writeRelayFrame(p.conn, mt, body)
}

// Close shuts the listener; in-flight goroutines exit on their own.
func (s *RelayServer) Close() error { return s.ln.Close() }

// ─── Client (peer-side) ────────────────────────────────────────────

// RelayClient is the peer-side dial helper. A peer that can't accept inbound
// connections (NAT, firewall) opens one of these to a relay and uses it to
// receive INCOMING frames + send FORWARD frames.
type RelayClient struct {
	conn   net.Conn
	pubkey [32]byte

	writeMu sync.Mutex
}

// DialRelay connects to a relay at addr, sends REGISTER + handles
// challenge-response (Sprint 3.5e BUG-W41), and returns a ready-to-use client.
//
// privkey wajib match pubkey (ed25519 keypair). Server kirim CHALLENGE nonce,
// client harus reply SIGNATURE = ed25519.Sign(privkey, nonce). Server verify
// — kalau invalid, connection di-close dengan error message.
//
// Caller responsible periodic Heartbeat() (recommended: every 30s) + reading Recv().
func DialRelay(ctx context.Context, addr string, pubkey [32]byte, privkey ed25519.PrivateKey, label string) (*RelayClient, error) {
	if len(privkey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key size: %d (need %d)", len(privkey), ed25519.PrivateKeySize)
	}
	var d net.Dialer
	d.Timeout = 10 * time.Second
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial relay %s: %w", addr, err)
	}
	body := make([]byte, 32+len(label))
	copy(body[:32], pubkey[:])
	copy(body[32:], label)
	if err := writeRelayFrame(conn, relayMsgRegister, body); err != nil {
		conn.Close()
		return nil, fmt.Errorf("register: %w", err)
	}

	// BUG-W41 fix: handle CHALLENGE → SIGNATURE.
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	mt, nonce, err := readRelayFrame(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read challenge: %w", err)
	}
	if mt == relayMsgError {
		conn.Close()
		return nil, fmt.Errorf("relay rejected: %s", string(nonce))
	}
	if mt != relayMsgChallenge || len(nonce) != 32 {
		conn.Close()
		return nil, fmt.Errorf("expected CHALLENGE (32-byte nonce), got mt=%d len=%d", mt, len(nonce))
	}
	sig := ed25519.Sign(privkey, nonce)
	if err := writeRelayFrame(conn, relayMsgSignature, sig); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send signature: %w", err)
	}
	return &RelayClient{conn: conn, pubkey: pubkey}, nil
}

// Forward asks the relay to deliver payload to the peer with the given pubkey.
// Payload is opaque to the relay (typically a NaCl-sealed transport frame).
func (c *RelayClient) Forward(toPubkey [32]byte, payload []byte) error {
	body := make([]byte, 32+len(payload))
	copy(body[:32], toPubkey[:])
	copy(body[32:], payload)
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	return writeRelayFrame(c.conn, relayMsgForward, body)
}

// Heartbeat sends a HEARTBEAT to refresh the relay's idle timer.
func (c *RelayClient) Heartbeat() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	return writeRelayFrame(c.conn, relayMsgHeartbeat, nil)
}

// Recv blocks until the next frame arrives. Returns (msgType, body, err).
// For INCOMING messages, body[:32] is the sender's pubkey and body[32:] is
// the opaque payload.
func (c *RelayClient) Recv() (byte, []byte, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(relayReadDeadln))
	return readRelayFrame(c.conn)
}

// Close shuts the connection.
func (c *RelayClient) Close() error { return c.conn.Close() }

// ─── Frame I/O ──────────────────────────────────────────────────────

func writeRelayFrame(w io.Writer, mt byte, body []byte) error {
	if len(body) > relayMaxFrame {
		return fmt.Errorf("frame too large: %d", len(body))
	}
	hdr := make([]byte, 5)
	hdr[0] = mt
	binary.BigEndian.PutUint32(hdr[1:5], uint32(len(body)))
	if _, err := w.Write(hdr); err != nil {
		return fmt.Errorf("relay: write header: %w", err)
	}
	if len(body) > 0 {
		if _, err := w.Write(body); err != nil {
			return fmt.Errorf("relay: write body: %w", err)
		}
	}
	return nil
}

func readRelayFrame(r io.Reader) (byte, []byte, error) {
	var hdr [5]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	mt := hdr[0]
	bodyLen := binary.BigEndian.Uint32(hdr[1:5])
	if bodyLen > relayMaxFrame {
		return 0, nil, errors.New("frame too large")
	}
	if bodyLen == 0 {
		return mt, nil, nil
	}
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return mt, body, nil
}
