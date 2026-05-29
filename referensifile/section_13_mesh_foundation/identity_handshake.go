// Package mesh — identity_handshake.go
//
// Identity card exchange antar peer. Setelah Discover() dapet peer URL dari
// seed, kernel call GET /v1/mesh/identity di peer itu untuk dapet:
//
//	{
//	  "peer_id":     "flowork-peer://<prefix>:<uuid>",
//	  "public_key":  "<base64 ed25519 32 bytes>",
//	  "kernel_version": "v1.x",
//	  "issued_at":   <unix sec>
//	}
//
// Hasilnya di-cache di Peer struct (kernel/mesh/peer_discover.go) supaya
// signature verify bisa langsung pakai cached pubkey tanpa re-fetch tiap
// CRDT sync.
//
// Per VISI_FINAL Pilar 3 — Zero Trust Peer Validation: "tiap data sync WAJIB
// ada cryptographic signature". Identity card = peer mengaku identitas-nya
// dengan pubkey, signed self-attestation.

package mesh

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flowork/kernel/kernel/identity"
)

// IdentityCard — payload yang di-return oleh /v1/mesh/identity.
//
// Self-attestation: PeerID di-derive dari PublicKey (lihat identity package),
// jadi cara verify-nya sederhana — Sign empty payload pake claimed pubkey,
// kalau sig match berarti owner pubkey memang punya private key sesuai.
//
// Untuk anti-replay sederhana: IssuedAt timestamp + signature dari nonce
// yang di-request caller. Phase C step 5 (trust score) akan tambah challenge
// nonce — sekarang MVP: trust kalau identity_card.public_key matches saat
// CRDT sync kemudian (signature pada data payload).
type IdentityCard struct {
	PeerID        string `json:"peer_id"`
	PublicKey     string `json:"public_key"`     // base64 std encoding ed25519 32 bytes
	KernelVersion string `json:"kernel_version,omitempty"`
	IssuedAt      int64  `json:"issued_at"`
}

// kernelVersion — populated by build (-ldflags -X). Default "dev".
var kernelVersion = "dev"

// LocalIdentityCard build identity card untuk kernel ini (untuk respond ke
// peer yang call /v1/mesh/identity).
func LocalIdentityCard() (*IdentityCard, error) {
	pid := identity.PeerID()
	pub := identity.PublicKey()
	if pid == "" || pub == nil {
		return nil, fmt.Errorf("mesh: local identity not initialized")
	}
	return &IdentityCard{
		PeerID:        pid,
		PublicKey:     base64.StdEncoding.EncodeToString(pub),
		KernelVersion: kernelVersion,
		IssuedAt:      time.Now().UTC().Unix(),
	}, nil
}

// FetchIdentityCard — call peer's /v1/mesh/identity endpoint, parse response.
//
// Timeout 10s. Return error kalau peer offline, response malformed, atau
// pubkey size salah. Caller wajib propagate ke Peer struct kalau sukses.
func FetchIdentityCard(ctx context.Context, baseURL string) (*IdentityCard, error) {
	u := strings.TrimRight(baseURL, "/") + "/v1/mesh/identity"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build req: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var card IdentityCard
	if err := json.Unmarshal(body, &card); err != nil {
		return nil, fmt.Errorf("parse identity card: %w", err)
	}

	// Validate pubkey decode + size.
	pubRaw, err := base64.StdEncoding.DecodeString(card.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode pubkey: %w", err)
	}
	if len(pubRaw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("pubkey size %d != %d", len(pubRaw), ed25519.PublicKeySize)
	}
	if !strings.HasPrefix(card.PeerID, "flowork-peer://") {
		return nil, fmt.Errorf("invalid peer_id format: %q", card.PeerID)
	}

	return &card, nil
}
