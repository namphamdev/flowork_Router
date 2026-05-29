package mesh

import (
	"context"
	"fmt"
	"sync"
)

// Transport is the plug-and-play contract (ADR-001 Tier 1) for mesh
// peer-to-peer communication. Implementations can register via init()
// in their own package; mesh core does not import concrete transports.
//
// Current registered transports:
//   - "v1"  (legacy: plaintext pubkey handshake + NaCl box framing)
//   - "v2"  (challenge-response handshake + nonce replay window)
//
// Future: QUIC, LoRa, Bluetooth, USB sneakernet — each as a new file
// in internal/mesh/ implementing Transport + self-registering.
type Transport interface {
	// Name is the stable identifier used in config/env/CLI (e.g. "v1", "v2").
	Name() string

	// ProtocolVersion is the integer version for ordering (v1=1, v2=2).
	// Used by compatibility negotiation — peer with higher version may
	// downgrade to match, never upgrade unilaterally.
	ProtocolVersion() int

	// Dial initiates an outbound encrypted session to peer at addr.
	// ourKeys is our KeyPair; peerPub is the peer's published public key
	// (obtained from mDNS TXT records via discovery.go).
	Dial(ctx context.Context, addr string, ourKeys *KeyPair, peerPub *[32]byte) (*Session, error)

	// Handshake completes the inbound handshake after Listener.Accept.
	// Returns a Session ready for Send/Recv, or error if handshake failed
	// (unknown peer, bad signature, protocol mismatch).
	Handshake(ctx context.Context, conn PeerConn, ourKeys *KeyPair, allowed AllowedPeers) (*Session, error)
}

// PeerConn is the minimal subset of net.Conn that Handshake needs.
// Kept small so mock transports (for testing) can satisfy it easily.
type PeerConn interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// AllowedPeers is a read-only view of the trusted peer keyset.
// Implementations take a Contains(pub) check only — no mutation.
type AllowedPeers interface {
	Contains(pub [32]byte) bool
}

// ─── Transport Registry ──────────────────────────────────────────────

var (
	transportsMu sync.RWMutex
	transports   = map[string]Transport{}
)

// RegisterTransport installs a transport implementation. Idempotent —
// re-register replaces. Typically called from init() of a file that
// provides the impl (e.g. transport.go registers "v1").
func RegisterTransport(t Transport) {
	transportsMu.Lock()
	defer transportsMu.Unlock()
	transports[t.Name()] = t
}

// GetTransport returns the registered transport by name, or nil if missing.
func GetTransport(name string) Transport {
	transportsMu.RLock()
	defer transportsMu.RUnlock()
	return transports[name]
}

// ListTransports returns all registered transport names sorted by
// protocol version descending (newest first).
func ListTransports() []string {
	transportsMu.RLock()
	defer transportsMu.RUnlock()
	// Manual insertion sort by ProtocolVersion desc, avoids importing "sort"
	names := make([]string, 0, len(transports))
	for n := range transports {
		names = append(names, n)
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && transports[names[j]].ProtocolVersion() > transports[names[j-1]].ProtocolVersion(); j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// DefaultTransport returns the preferred transport based on env
// FLOWORK_MESH_PROTO (values: "v1", "v2", ...) — falls back to newest
// registered when unset.
func DefaultTransport(override string) (Transport, error) {
	if override != "" {
		if t := GetTransport(override); t != nil {
			return t, nil
		}
		return nil, fmt.Errorf("mesh transport %q not registered (available: %v)", override, ListTransports())
	}
	names := ListTransports()
	if len(names) == 0 {
		return nil, fmt.Errorf("no mesh transports registered")
	}
	return GetTransport(names[0]), nil
}
