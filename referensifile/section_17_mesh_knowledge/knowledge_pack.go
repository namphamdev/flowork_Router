// Package mesh — knowledge_pack.go: M8 knowledge-only sharing helpers.
//
// Per AMENDMENTS-V1 W-6 (default mode flip): drawer text yang udah PII-
// redacted via L4 di-share via mesh sebagai KnowledgePacket type=drawer.
// JANGAN share V4 weight delta by default (privacy risk).
//
// Weight delta sharing = optional advanced opt-in (settings DB
// MESH_SHARE_WEIGHT_DELTA=true). Default false.
//
// Helper di sini = build & sign drawer KnowledgePacket dari brain content.

package mesh

import (
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/google/uuid"
)

// PackDrawerKnowledge build signed KnowledgePacket dari drawer content.
//
// Caller wajib pass:
//   - content: drawer text (sudah PII-redacted oleh caller, atau biarkan
//     filter L4 reject saat receive di peer lain)
//   - selfPubKey + selfPriv: self-issued keypair (kernel/identity)
//   - licenseID: machine fingerprint (kernel/mesh.SelfIssuedLicense)
//   - amplitude: confidence 0..1
//   - parentID: chain proof (id packet sebelumnya, "" untuk first)
//
// Result: KnowledgePacket ready broadcast via M6 gossip.
func PackDrawerKnowledge(
	content string,
	selfPubKey []byte,
	selfPriv ed25519.PrivateKey,
	licenseID string,
	amplitude float64,
	parentID string,
) (KnowledgePacket, error) {
	if content == "" {
		return KnowledgePacket{}, errors.New("empty content")
	}
	if len(selfPubKey) != ed25519.PublicKeySize {
		return KnowledgePacket{}, errors.New("invalid pubkey size")
	}
	if len(selfPriv) != ed25519.PrivateKeySize {
		return KnowledgePacket{}, errors.New("invalid privkey size")
	}
	p := KnowledgePacket{
		ID:           uuid.New(),
		Type:         PacketTypeDrawer,
		Payload:      []byte(content),
		AuthorPubKey: selfPubKey,
		LicenseID:    licenseID,
		ParentID:     parentID,
		Amplitude:    clamp(amplitude, 0, 1),
		Timestamp:    time.Now().UTC(),
		HopCount:     0,
	}
	p.Signature = SignPacket(p, selfPriv)
	return p, nil
}

// IsWeightDeltaSharingEnabled — read settings flag MESH_SHARE_WEIGHT_DELTA.
//
// Default false (knowledge-only mode per AMENDMENTS-V1 W-6).
//
// Set true via:
//   curl -X POST /v1/settings/MESH_SHARE_WEIGHT_DELTA -d 'true'
//   (tools/settings ada built-in di kernel)
func IsWeightDeltaSharingEnabled() bool {
	// Default false. Caller (M8 trainer) check ini sebelum broadcast delta.
	// Stub for now — wire via settings.Shared() di future PR.
	return false
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
