// Package mesh — knowledge_share.go
//
// Phase C step 4: P2P knowledge sync OPT-IN dengan PII filter.
//
// Per VISI_FINAL Pilar 3:
//
//	"Knowledge (opt-in, anonymized, filter PII strip)"
//
// Flow:
//  1. User Ayah turn on di Settings tab: MESH_KNOWLEDGE_SHARE_ENABLED=true
//  2. Peer A call GET /v1/mesh/knowledge/share di kernel B
//  3. Kernel B cek opt-in flag → kalau OFF, return 403 "opt-out"
//  4. Kalau ON: build signed envelope berisi knowledge entries (data
//     fields di-redact via privacy.RedactMap)
//  5. Kernel A verify signature → store ke local pool dengan submitter_id =
//     peer B's PeerID
//
// Signed envelope format mirip ToolManifest. Canonical payload =
// peer_id\n + issued_at\n + json.Marshal(entries) — diff dari tool manifest
// karena entries punya struktur lebih kompleks.
//
// Endpoint return-nya HTTP 403 kalau opt-out, jadi peer caller tau ngga
// usah retry.

package mesh

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flowork/kernel/kernel/identity"
	"github.com/flowork/kernel/kernel/privacy"
	"github.com/flowork/kernel/kernel/settings"
)

// SettingsKeyKnowledgeShareEnabled — opt-in flag di settings DB.
//
// Default OFF. User Ayah toggle via GUI Settings tab → toggle ON
// kalau pengen kontribusi ke mesh.
const SettingsKeyKnowledgeShareEnabled = "MESH_KNOWLEDGE_SHARE_ENABLED"

// ErrShareOptedOut — kernel ngga share knowledge (opt-out).
var ErrShareOptedOut = errors.New("mesh knowledge share: opted out")

// KnowledgeEntry — minimal subset dari kernel/knowledge.Entry yang aman
// di-share. Sengaja ngga embed Entry langsung biar SubmitterID asli (yang
// mungkin user-private) ngga bocor — kita over-write dengan PeerID kernel.
type KnowledgeEntry struct {
	Type string         `json:"type"`           // qa_pair | fact | correction | skill_demo
	Data map[string]any `json:"data"`           // PII-redacted
	Tags []string       `json:"tags,omitempty"` // anonymous tags only
}

// KnowledgeEnvelope — signed payload untuk /v1/mesh/knowledge/share response.
type KnowledgeEnvelope struct {
	PeerID    string           `json:"peer_id"`
	IssuedAt  int64            `json:"issued_at"`
	Entries   []KnowledgeEntry `json:"entries"`
	Signature string           `json:"signature"` // base64 ed25519
}

// IsKnowledgeShareEnabled — read settings flag. Empty/false/0/no → false.
func IsKnowledgeShareEnabled() bool {
	store := settings.Shared()
	if store == nil {
		return false
	}
	v, _ := store.Get(SettingsKeyKnowledgeShareEnabled)
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

// canonicalEnvelope — bytes yang di-sign / verify.
//
// Format:
//
//	peer_id\n
//	issued_at\n
//	{json entries — deterministic via stable marshal}
//
// Note: Go's json.Marshal map[string]any TIDAK deterministic ordering
// keys-nya. Untuk MVP, kita assume entries.Data udah pre-canonicalized
// (mostly fine kalau Entries di-build via Redact dari Entry yang konsisten).
// Phase F mungkin perlu canonical JSON (RFC 8785) kalau cross-impl mesh.
func (e *KnowledgeEnvelope) canonicalPayload() ([]byte, error) {
	var b strings.Builder
	b.WriteString(e.PeerID)
	b.WriteByte('\n')
	fmt.Fprintf(&b, "%d\n", e.IssuedAt)
	entriesJSON, err := json.Marshal(e.Entries)
	if err != nil {
		return nil, fmt.Errorf("marshal entries: %w", err)
	}
	b.Write(entriesJSON)
	return []byte(b.String()), nil
}

// BuildLocalKnowledgeEnvelope — pack entries dengan PII filter + signature.
//
// `entries` adalah raw entry list dari local knowledge pool — caller
// (handler) yang bertanggung jawab fetch dari kernel.knowledge.
//
// Filter PII otomatis pada Data field. Tags + Type ditinggalkan apa adanya
// (assumed tidak mengandung PII).
//
// Return ErrShareOptedOut kalau settings DB flag OFF.
func BuildLocalKnowledgeEnvelope(entries []KnowledgeEntry) (*KnowledgeEnvelope, error) {
	if !IsKnowledgeShareEnabled() {
		return nil, ErrShareOptedOut
	}
	pid := identity.PeerID()
	if pid == "" {
		return nil, fmt.Errorf("knowledge envelope: identity not initialized")
	}

	// PII redact pada Data field tiap entry.
	cleaned := make([]KnowledgeEntry, len(entries))
	for i, e := range entries {
		cleaned[i] = KnowledgeEntry{
			Type: e.Type,
			Data: privacy.RedactMap(e.Data),
			Tags: e.Tags,
		}
	}

	env := &KnowledgeEnvelope{
		PeerID:   pid,
		IssuedAt: time.Now().UTC().Unix(),
		Entries:  cleaned,
	}
	payload, err := env.canonicalPayload()
	if err != nil {
		return nil, err
	}
	sig, err := identity.Sign(payload)
	if err != nil {
		return nil, fmt.Errorf("sign envelope: %w", err)
	}
	env.Signature = base64.StdEncoding.EncodeToString(sig)
	return env, nil
}

// VerifyKnowledgeEnvelope — check signature pakai peer's pubkey base64.
func VerifyKnowledgeEnvelope(e *KnowledgeEnvelope, peerPublicKey string) error {
	if e == nil {
		return errors.New("verify: nil envelope")
	}
	pubRaw, err := base64.StdEncoding.DecodeString(peerPublicKey)
	if err != nil {
		return fmt.Errorf("decode peer pubkey: %w", err)
	}
	if len(pubRaw) != ed25519.PublicKeySize {
		return fmt.Errorf("peer pubkey size %d != %d", len(pubRaw), ed25519.PublicKeySize)
	}
	sigRaw, err := base64.StdEncoding.DecodeString(e.Signature)
	if err != nil {
		return fmt.Errorf("decode sig: %w", err)
	}
	payload, err := e.canonicalPayload()
	if err != nil {
		return err
	}
	return identity.Verify(ed25519.PublicKey(pubRaw), payload, sigRaw)
}
