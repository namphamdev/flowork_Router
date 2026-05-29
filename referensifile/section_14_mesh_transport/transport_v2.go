package mesh

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/nacl/box"
)

// transportV2 implements a challenge-response handshake that proves the
// peer holds the private key matching the advertised public key.
// Closes Gemini audit Bug 7.1 (Zero-Challenge Handshake).
//
// Wire format (all sizes big-endian):
//
//  1. Client → Server  : 1 byte version (0x02) | 32 bytes client pubkey
//  2. Server verifies pubkey ∈ allowedPeers. If unknown, close.
//  3. Server → Client  : 32 bytes server pubkey | 32 bytes random challenge C_s
//  4. Client → Server  : box.Seal(C_s, nonce_c, server_pub, client_priv)
//     framed as | 24 nonce | 48 ciphertext (32 + overhead) |
//  5. Server decrypts, compares plaintext to original C_s. Mismatch = reject.
//  6. Server → Client  : 24 nonce | box.Seal(hello_tag, nonce_s, client_pub, server_priv)
//     where hello_tag = "flowork-v2-ok" (14 bytes + padding)
//  7. Client decrypts to confirm server also has matching private key.
//
// After handshake, the regular Session framing (24-byte nonce + box.Seal) kicks in.
// Replay window + nonce cache (from Bug 7.2 fix) carries over automatically.
type transportV2 struct{}

const (
	v2Version       byte = 0x02
	v2ChallengeSize      = 32
	v2HelloTag           = "flowork-v2-ok\x00"
)

func (transportV2) Name() string         { return "v2" }
func (transportV2) ProtocolVersion() int { return 2 }

func (transportV2) Dial(ctx context.Context, addr string, our *KeyPair, peerPub *[32]byte) (*Session, error) {
	var d net.Dialer
	d.Timeout = handshakeTimeout
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("v2 dial: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))

	// Step 1: send version + our pubkey
	header := make([]byte, 1+32)
	header[0] = v2Version
	copy(header[1:], our.Public[:])
	if _, err := conn.Write(header); err != nil {
		conn.Close()
		return nil, fmt.Errorf("v2 send header: %w", err)
	}

	// Step 3: read server pubkey + challenge
	var serverPub [32]byte
	if _, err := io.ReadFull(conn, serverPub[:]); err != nil {
		conn.Close()
		return nil, fmt.Errorf("v2 read server pubkey: %w", err)
	}
	// Cross-check: if caller expected a specific peerPub, reject impostor.
	if peerPub != nil && *peerPub != serverPub {
		conn.Close()
		return nil, errors.New("v2 server pubkey mismatch (expected peer != actual)")
	}
	challenge := make([]byte, v2ChallengeSize)
	if _, err := io.ReadFull(conn, challenge); err != nil {
		conn.Close()
		return nil, fmt.Errorf("v2 read challenge: %w", err)
	}

	// Step 4: seal challenge back with our private key
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		conn.Close()
		return nil, fmt.Errorf("v2 nonce: %w", err)
	}
	sealed := box.Seal(nil, challenge, &nonce, &serverPub, our.Private)
	if err := writeFrame(conn, nonce[:], sealed); err != nil {
		conn.Close()
		return nil, fmt.Errorf("v2 send sealed challenge: %w", err)
	}

	// Step 6: read server's hello tag, confirms server has matching priv
	helloNonce, helloCipher, err := readFrame(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("v2 read hello: %w", err)
	}
	var hn [24]byte
	copy(hn[:], helloNonce)
	helloPlain, ok := box.Open(nil, helloCipher, &hn, &serverPub, our.Private)
	if !ok || string(helloPlain) != v2HelloTag {
		conn.Close()
		return nil, errors.New("v2 server hello invalid — possible MITM or replay")
	}

	_ = conn.SetDeadline(time.Time{})
	return &Session{conn: conn, ourPriv: our.Private, peerPub: &serverPub}, nil
}

func (transportV2) Handshake(ctx context.Context, pc PeerConn, our *KeyPair, allowed AllowedPeers) (*Session, error) {
	conn, _ := pc.(net.Conn)
	if conn != nil {
		_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))
	}

	// Step 1: read version + client pubkey
	header := make([]byte, 1+32)
	if _, err := io.ReadFull(pc, header); err != nil {
		return nil, fmt.Errorf("v2 read header: %w", err)
	}
	if header[0] != v2Version {
		return nil, fmt.Errorf("v2 protocol mismatch: got 0x%02x want 0x%02x", header[0], v2Version)
	}
	var clientPub [32]byte
	copy(clientPub[:], header[1:])
	if !allowed.Contains(clientPub) {
		return nil, errors.New("v2 unknown peer")
	}

	// Step 3: send server pubkey + random challenge
	challenge := make([]byte, v2ChallengeSize)
	if _, err := rand.Read(challenge); err != nil {
		return nil, fmt.Errorf("v2 gen challenge: %w", err)
	}
	if _, err := pc.Write(our.Public[:]); err != nil {
		return nil, fmt.Errorf("v2 send pubkey: %w", err)
	}
	if _, err := pc.Write(challenge); err != nil {
		return nil, fmt.Errorf("v2 send challenge: %w", err)
	}

	// Step 5: read sealed challenge, verify
	nonce, ciphertext, err := readFrame(pc)
	if err != nil {
		return nil, fmt.Errorf("v2 read sealed challenge: %w", err)
	}
	var n [24]byte
	copy(n[:], nonce)
	plain, ok := box.Open(nil, ciphertext, &n, &clientPub, our.Private)
	if !ok {
		return nil, errors.New("v2 decrypt failed — client key mismatch")
	}
	if !constantTimeEqual(plain, challenge) {
		return nil, errors.New("v2 challenge mismatch — possible replay or impostor")
	}

	// Step 6: send hello tag sealed with our priv to prove mutual possession
	var helloNonce [24]byte
	if _, err := rand.Read(helloNonce[:]); err != nil {
		return nil, fmt.Errorf("v2 hello nonce: %w", err)
	}
	helloSealed := box.Seal(nil, []byte(v2HelloTag), &helloNonce, &clientPub, our.Private)
	if err := writeFrame(pc, helloNonce[:], helloSealed); err != nil {
		return nil, fmt.Errorf("v2 send hello: %w", err)
	}

	if conn != nil {
		_ = conn.SetDeadline(time.Time{})
	}
	return &Session{conn: conn, ourPriv: our.Private, peerPub: &clientPub}, nil
}

// writeFrame emits: [4 BE length][24 nonce][ciphertext]
func writeFrame(w io.Writer, nonce, ciphertext []byte) error {
	if len(nonce) != 24 {
		return fmt.Errorf("nonce must be 24 bytes, got %d", len(nonce))
	}
	frame := make([]byte, 4+24+len(ciphertext))
	binary.BigEndian.PutUint32(frame[:4], uint32(24+len(ciphertext)))
	copy(frame[4:28], nonce)
	copy(frame[28:], ciphertext)
	_, err := w.Write(frame)
	if err != nil {
		return fmt.Errorf("mesh frame write: %w", err)
	}
	return nil
}

// readFrame reads the inverse.
func readFrame(r io.Reader) (nonce, ciphertext []byte, err error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, nil, err
	}
	total := binary.BigEndian.Uint32(lenBuf[:])
	if total < 24 || total > maxFrameSize {
		return nil, nil, fmt.Errorf("frame size out of range: %d", total)
	}
	nonceBuf := make([]byte, 24)
	if _, err := io.ReadFull(r, nonceBuf); err != nil {
		return nil, nil, err
	}
	lr := io.LimitReader(r, int64(total-24))
	cipherBuf, err := io.ReadAll(lr)
	if err != nil {
		return nil, nil, err
	}
	if len(cipherBuf) != int(total-24) {
		return nil, nil, io.ErrUnexpectedEOF
	}
	return nonceBuf, cipherBuf, nil
}

// constantTimeEqual compares two byte slices in constant time to prevent
// timing side-channel on challenge verification.
func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func init() {
	RegisterTransport(transportV2{})
}
