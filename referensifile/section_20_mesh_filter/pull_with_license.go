// Package mesh — kernel/mesh/pull_with_license.go
//
// PullWithLicense wraps Pull dengan license-bound AES-256-GCM chunk
// decryption. Caller-side abstraction: brain.LoadWeights atau GUI download
// flow ngga perlu tau soal license internal — pure clean wrapper.
//
// Pattern:
//
//	res, err := mesh.PullWithLicense(ctx, peer, "brain_v4_q4")
//	if errors.Is(err, license.ErrNoLicense) {
//	    // Tier edge tanpa weight access — clear UX, bukan bug
//	}
//
// Threat model (per docs/06-NETWORK.md § Anti-leak):
//   1. Peer stores AES-256-GCM ciphertext + sha256-of-ciphertext hashes
//   2. Manifest Merkle root = atas chunk-of-ciphertext hashes
//   3. Pull verifies hash + Merkle root (no plaintext access during transit)
//   4. License-holder operator decrypts post-fetch (key bound ke license claim)
//
// Single integration point license↔mesh: kalau license design berubah
// (mis. tier-based decrypt, key rotation), cuma file ini yang touch.

package mesh

import (
	"crypto/aes"
	"crypto/cipher"
	"context"
	"errors"
	"fmt"

	"github.com/flowork/kernel/kernel/license"
)

// gcmNonceSize standard AES-GCM nonce — 12 bytes.
const gcmNonceSize = 12

// PullWithLicense fetch + verify + decrypt full weight set dari peer.
// Single-call abstraction untuk caller (brain.LoadWeights, GUI download).
//
// Decryption: AES-256-GCM dengan key dari license.DecryptKey().
// Wire format chunk: nonce (12 byte) || ciphertext || tag (16 byte).
//
// Errors:
//   - license error (no license, tier ngga punya weight access, expired)
//   - all errors dari Pull (manifest, peer unreachable, hash mismatch, dll)
//   - AES init / GCM open fail (corrupted chunk OR wrong key)
func PullWithLicense(ctx context.Context, peer Peer, version string) (PullResult, error) {
	// 1. Resolve license-bound AES key.
	key, err := license.DecryptKey()
	if err != nil {
		return PullResult{}, fmt.Errorf("mesh.PullWithLicense: license: %w", err)
	}
	if len(key) != 32 {
		return PullResult{}, fmt.Errorf("mesh.PullWithLicense: key len %d (expect 32 for AES-256)", len(key))
	}

	// 2. Build AES-GCM cipher (one-shot init, reused per-chunk via closure).
	block, err := aes.NewCipher(key)
	if err != nil {
		return PullResult{}, fmt.Errorf("mesh.PullWithLicense: aes init: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return PullResult{}, fmt.Errorf("mesh.PullWithLicense: gcm init: %w", err)
	}

	// 3. Decrypt fn — wrapped over aead. Chunk format: nonce||ciphertext||tag.
	decrypt := func(chunkBytes []byte) ([]byte, error) {
		if len(chunkBytes) < gcmNonceSize+aead.Overhead() {
			return nil, errors.New("chunk too small (missing nonce or tag)")
		}
		nonce := chunkBytes[:gcmNonceSize]
		ciphertext := chunkBytes[gcmNonceSize:]
		plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil, fmt.Errorf("aead open: %w (corrupted chunk OR wrong key)", err)
		}
		return plaintext, nil
	}

	// 4. Delegate ke Pull dengan decryptFn injected.
	return Pull(ctx, peer.URL, version, decrypt)
}
