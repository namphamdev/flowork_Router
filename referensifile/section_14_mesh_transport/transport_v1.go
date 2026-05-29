package mesh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// transportV1 is the legacy NaCl-box transport (pre-ADR-001).
// Protocol:
//
//	client → server: 32-byte plaintext public key
//	server verifies pub ∈ allowedPeers, then both exchange box-sealed frames.
//
// Known limitation: server has no proof that the client holds the
// matching private key (challenge-response lives in v2).
type transportV1 struct{}

func (transportV1) Name() string         { return "v1" }
func (transportV1) ProtocolVersion() int { return 1 }

func (transportV1) Dial(ctx context.Context, addr string, our *KeyPair, peerPub *[32]byte) (*Session, error) {
	var d net.Dialer
	d.Timeout = handshakeTimeout
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("v1 dial %s: %w", addr, err)
	}
	// Send our public key as first 32 bytes (legacy plaintext handshake).
	if _, err := conn.Write(our.Public[:]); err != nil {
		conn.Close()
		return nil, fmt.Errorf("v1 write pubkey: %w", err)
	}
	return &Session{conn: conn, ourPriv: our.Private, peerPub: peerPub}, nil
}

func (transportV1) Handshake(ctx context.Context, conn PeerConn, our *KeyPair, allowed AllowedPeers) (*Session, error) {
	// Wrap PeerConn into net.Conn view for deadline control.
	netConn, _ := conn.(net.Conn)
	if netConn != nil {
		_ = netConn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	}
	var pub [32]byte
	if _, err := io.ReadFull(conn, pub[:]); err != nil {
		return nil, fmt.Errorf("v1 read pubkey: %w", err)
	}
	if !allowed.Contains(pub) {
		return nil, errors.New("v1 unknown peer")
	}
	return &Session{conn: netConn, ourPriv: our.Private, peerPub: &pub}, nil
}

func init() {
	RegisterTransport(transportV1{})
}
