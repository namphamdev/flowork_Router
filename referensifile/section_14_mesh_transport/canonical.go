// Package mesh — canonical.go: deterministic byte encoding untuk signed
// payload (M4 L1 + M6 packet sign).
//
// Per AMENDMENTS-V1 W-2 CRITICAL: JANGAN sign json.Marshal output —
// non-deterministic key ordering = false-reject signature cross-platform.
// Sign target = sha256(CanonicalBytes(p)).
//
// Format KnowledgePacket canonical bytes (deterministic):
//
//	type      || 0x00 ||
//	payload   || 0x00 ||
//	timestamp_unixnano_be (8 bytes) || 0x00 ||
//	parent_id || 0x00 ||
//	hop_count_be32 (4 bytes)        || 0x00 ||
//	expires_at_unixnano_be (8 bytes, 0 kalau nil)
//
// Hop count + expires_at INCLUDED di canonical supaya peer ngga bisa edit
// hop count atau ttl tanpa invalidate signature.

package mesh

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
)

// CanonicalBytes deterministic byte encoding untuk packet signature.
//
// Layout fixed di semua platform — Go map ordering / OS encoding diff
// tidak affect output.
func CanonicalBytes(p KnowledgePacket) []byte {
	var buf []byte
	buf = append(buf, []byte(p.Type)...)
	buf = append(buf, 0x00)
	buf = append(buf, p.Payload...)
	buf = append(buf, 0x00)

	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(p.Timestamp.UTC().UnixNano()))
	buf = append(buf, ts[:]...)
	buf = append(buf, 0x00)

	buf = append(buf, []byte(p.ParentID)...)
	buf = append(buf, 0x00)

	var hop [4]byte
	binary.BigEndian.PutUint32(hop[:], uint32(p.HopCount))
	buf = append(buf, hop[:]...)
	buf = append(buf, 0x00)

	var expires [8]byte
	if p.ExpiresAt != nil {
		binary.BigEndian.PutUint64(expires[:], uint64(p.ExpiresAt.UTC().UnixNano()))
	}
	buf = append(buf, expires[:]...)

	return buf
}

// SignPacket sign canonical bytes hash dengan private key.
// Caller harus set Signature di p setelah call ini.
func SignPacket(p KnowledgePacket, priv ed25519.PrivateKey) []byte {
	canonical := CanonicalBytes(p)
	hash := sha256.Sum256(canonical)
	return ed25519.Sign(priv, hash[:])
}

// VerifyPacketSignature verify canonical bytes hash signature.
//
// Return true kalau valid. False kalau invalid signature, bad pubkey size,
// atau canonical bytes mismatch.
func VerifyPacketSignature(p KnowledgePacket, pubkey []byte) bool {
	if len(pubkey) != ed25519.PublicKeySize {
		return false
	}
	if len(p.Signature) != ed25519.SignatureSize {
		return false
	}
	canonical := CanonicalBytes(p)
	hash := sha256.Sum256(canonical)
	return ed25519.Verify(ed25519.PublicKey(pubkey), hash[:], p.Signature)
}
