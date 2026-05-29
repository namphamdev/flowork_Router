// Package mesh — tool_manifest.go
//
// Signed tool manifest exchange antar peer. Peer A bisa nanya peer B "tools
// apa aja yang lo punya?" dengan call GET /v1/mesh/tools/manifest. Response
// adalah signed JSON: list tool name yang ke-register di peer B + signature
// Ed25519 dari peer B's identity.
//
// Verifier (peer A) cek signature pakai pubkey dari IdentityCard yang udah
// di-fetch saat handshake. Kalau valid, peer B mengaku punya tools tsb.
//
// Per VISI_FINAL Pilar 3 — Anti-Poisoning:
//
//	"Tool manifest signed by Master Key (Ayah) → trusted as `master`.
//	 Tool manifest signed by user kernel → tagged `community`, butuh
//	 trust score dulu sebelum di-pull/install."
//
// Phase C step 3 ini SHARE manifest doang (read-only audit). PULL/INSTALL
// tool dari peer = step 5+ (perlu sandbox, version resolution, trust gate).
//
// Canonical payload format (untuk signature reproducibility):
//
//	{peer_id}\n{issued_at}\n{tool_name_1}\n{tool_name_2}\n...
//
// Tools sorted alfabetik (matches tools.List()), separated by \n. Tanda
// tangan menutupi seluruh canonical text.

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
	"github.com/flowork/kernel/kernel/tools"
)

// ToolManifest — payload yang di-share via /v1/mesh/tools/manifest.
type ToolManifest struct {
	PeerID    string   `json:"peer_id"`
	IssuedAt  int64    `json:"issued_at"`
	Tools     []string `json:"tools"`     // sorted tool names
	Signature string   `json:"signature"` // base64 ed25519
}

// canonicalPayload — bytes yang di-sign / di-verify. Format:
//
//	peer_id\n
//	issued_at\n
//	tool1\n
//	tool2\n
//	...
//
// Deterministic: assume Tools sudah sorted (List() returns sorted).
func (m *ToolManifest) canonicalPayload() []byte {
	var b strings.Builder
	b.WriteString(m.PeerID)
	b.WriteByte('\n')
	fmt.Fprintf(&b, "%d\n", m.IssuedAt)
	for _, t := range m.Tools {
		b.WriteString(t)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// BuildLocalToolManifest — generate signed manifest tools yang ke-register
// di kernel ini. Return error kalau identity belum di-Ensure.
func BuildLocalToolManifest() (*ToolManifest, error) {
	pid := identity.PeerID()
	if pid == "" {
		return nil, fmt.Errorf("mesh tool manifest: identity not initialized")
	}
	m := &ToolManifest{
		PeerID:   pid,
		IssuedAt: time.Now().UTC().Unix(),
		Tools:    tools.List(), // sorted
	}
	sig, err := identity.Sign(m.canonicalPayload())
	if err != nil {
		return nil, fmt.Errorf("sign manifest: %w", err)
	}
	m.Signature = base64.StdEncoding.EncodeToString(sig)
	return m, nil
}

// VerifyToolManifest — cek manifest signature pakai peer's pubkey
// (peerPublicKey base64 dari IdentityCard). Return nil = valid.
func VerifyToolManifest(m *ToolManifest, peerPublicKey string) error {
	if m == nil {
		return fmt.Errorf("verify: nil manifest")
	}
	pubRaw, err := base64.StdEncoding.DecodeString(peerPublicKey)
	if err != nil {
		return fmt.Errorf("decode peer pubkey: %w", err)
	}
	if len(pubRaw) != ed25519.PublicKeySize {
		return fmt.Errorf("peer pubkey size %d != %d", len(pubRaw), ed25519.PublicKeySize)
	}
	sigRaw, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	return identity.Verify(ed25519.PublicKey(pubRaw), m.canonicalPayload(), sigRaw)
}

// FetchPeerToolManifest — pull signed manifest dari peer. Caller usually
// follows up dengan VerifyToolManifest pakai peer's pubkey dari IdentityCard.
//
// Timeout 15s (manifest bisa agak besar kalau peer punya banyak tool).
// Limit body 1MB (anti-DoS).
func FetchPeerToolManifest(ctx context.Context, baseURL string) (*ToolManifest, error) {
	u := strings.TrimRight(baseURL, "/") + "/v1/mesh/tools/manifest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build req: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var m ToolManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.PeerID == "" || len(m.Tools) == 0 || m.Signature == "" {
		return nil, fmt.Errorf("manifest fields incomplete (peer_id=%q tools=%d sig=%d)",
			m.PeerID, len(m.Tools), len(m.Signature))
	}
	return &m, nil
}
