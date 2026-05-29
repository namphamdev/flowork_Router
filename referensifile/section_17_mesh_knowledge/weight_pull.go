// Package mesh — kernel/mesh/weight_pull.go
//
// Pull side: fetch manifest dari peer + chunk fetch + Merkle verify +
// (optional) decrypt via license-bound key. Save ke local weight dir.
//
// Caller pattern (kernel boot atau on-demand brain.LoadWeights):
//
//	dec := func(b []byte) ([]byte, error) { return license.DecryptKey(b) }
//	res, err := mesh.Pull(ctx, peerURL, "brain_v4_q4", dec)
//	if err != nil { return err }
//	// Manifest + chunks now di local <data>/weights/<version>/
//
// License integration: opus-3 expose kernel/license/decrypt_key.go stub
// (returns AES-256 key dari encrypted manifest.EncryptedKey). Pull wire
// via decryptFn parameter — default no-op kalau license belum ada.
//
// Anti-tampering layers (per REVISIONS Fix #6):
//   1. Per-chunk SHA-256 hash verify
//   2. Final Merkle root match
//   3. AES-256 decrypt (kalau encrypted)
//   4. Phase J: Ed25519 manifest signature verify (defer, license-bound)

package mesh

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PullResult ringkasan operasi Pull untuk caller telemetry.
type PullResult struct {
	Version    string
	ChunkCount int
	TotalBytes int64
	DurationMs int
	Peer       string
}

// DecryptFn signature untuk per-chunk decryption — caller (typically
// PullWithLicense wrapper) wire AES-GCM open dengan license-bound key.
//
// Pull flow: fetch ciphertext → hash-verify (over ciphertext) → size match →
// decryptFn(ciphertext) → save plaintext. Hash + Merkle root semantics
// remain over ciphertext (peer-stored bytes).
//
// nil decryptFn → no decryption, save ciphertext as-is (Phase E2-A1
// behavior, dev/test).
type DecryptFn func(ciphertext []byte) (plaintext []byte, err error)

// pullTimeout per-chunk download cap. Manifest (small JSON) cepet, chunk
// (4 MiB) butuh waktu di koneksi lambat.
const (
	pullManifestTimeout = 10 * time.Second
	pullChunkTimeout    = 60 * time.Second // 4 MiB / kbps low = ~30s, double for safety
)

// Pull fetch full weight set dari peer URL. Manifest + chunks + verify +
// decrypt (kalau ada) + save local. Idempotent — chunk sudah ada di local
// di-skip download (resume support).
//
// Errors:
//   - context cancel/timeout
//   - manifest fetch HTTP non-2xx
//   - manifest parse fail
//   - chunk count = 0 (empty manifest)
//   - per-chunk hash mismatch (poisoning detected)
//   - Merkle root mismatch (manifest tampered)
//   - decrypt fail
//   - local disk write fail
func Pull(ctx context.Context, peerURL, version string, decryptFn DecryptFn) (PullResult, error) {
	started := time.Now()
	res := PullResult{Version: version, Peer: peerURL}

	if peerURL == "" {
		return res, errors.New("Pull: peerURL empty")
	}
	if !validVersion(version) {
		return res, fmt.Errorf("Pull: invalid version %q", version)
	}

	// 1. Fetch manifest.
	manifest, err := fetchManifest(ctx, peerURL, version)
	if err != nil {
		return res, fmt.Errorf("Pull: manifest: %w", err)
	}
	if len(manifest.Chunks) == 0 {
		return res, errors.New("Pull: empty manifest (no chunks)")
	}

	// 2. Verify Merkle root match claimed.
	hashes := make([]string, len(manifest.Chunks))
	for i, c := range manifest.Chunks {
		hashes[i] = c.Hash
	}
	computedRoot, err := MerkleRoot(hashes)
	if err != nil {
		return res, fmt.Errorf("Pull: compute merkle: %w", err)
	}
	if computedRoot != manifest.MerkleRoot {
		return res, fmt.Errorf("Pull: merkle root mismatch (computed %s, manifest %s) — possible tampering",
			computedRoot, manifest.MerkleRoot)
	}

	// 3. Fetch + verify + (optional) decrypt + save each chunk.
	// Resume-aware: chunks sudah ada di local skip download (idempotent).
	for _, c := range manifest.Chunks {
		// Skip kalau chunk udah ada di local.
		if r, _, err := ServeChunk(version, c.Hash); err == nil {
			r.Close()
			res.ChunkCount++
			res.TotalBytes += c.Size
			continue
		}

		ciphertext, err := fetchChunk(ctx, peerURL, c.Hash)
		if err != nil {
			return res, fmt.Errorf("Pull: chunk %s: %w", c.Hash, err)
		}

		// Verify per-chunk hash (over ciphertext, peer-stored bytes).
		sum := sha256.Sum256(ciphertext)
		if hex.EncodeToString(sum[:]) != c.Hash {
			return res, fmt.Errorf("Pull: chunk hash mismatch %s — peer poisoning?", c.Hash)
		}
		if int64(len(ciphertext)) != c.Size {
			return res, fmt.Errorf("Pull: chunk %s size mismatch (got %d, want %d)",
				c.Hash, len(ciphertext), c.Size)
		}

		// Optional decrypt — wrapper (PullWithLicense) injects AES-GCM.
		// nil decryptFn → save ciphertext as-is (dev/test path).
		var toSave []byte
		if decryptFn != nil {
			plaintext, derr := decryptFn(ciphertext)
			if derr != nil {
				return res, fmt.Errorf("Pull: decrypt chunk %s: %w", c.Hash, derr)
			}
			toSave = plaintext
		} else {
			toSave = ciphertext
		}

		// SaveChunk verifies hash — caveat: ngga match plaintext kalau
		// decryptFn != nil. Adjust SaveChunk pattern: pakai SaveChunkRaw
		// untuk plaintext (post-decrypt). Defense in depth lost di flow
		// terdesign untuk acceptable trade-off (hash verified above).
		if decryptFn != nil {
			if err := saveChunkRaw(version, c.Hash, toSave); err != nil {
				return res, fmt.Errorf("Pull: save plaintext chunk %s: %w", c.Hash, err)
			}
		} else {
			if err := SaveChunk(version, c.Hash, toSave); err != nil {
				return res, fmt.Errorf("Pull: save chunk %s: %w", c.Hash, err)
			}
		}
		res.ChunkCount++
		res.TotalBytes += c.Size
	}

	// 5. Save manifest local (mark version present for HasLocalVersion).
	if err := SaveLocalManifest(manifest); err != nil {
		return res, fmt.Errorf("Pull: save manifest: %w", err)
	}

	res.DurationMs = int(time.Since(started).Milliseconds())
	return res, nil
}

// fetchManifest GET /v1/weight/manifest/{version}.
func fetchManifest(ctx context.Context, peerURL, version string) (Manifest, error) {
	cctx, cancel := context.WithTimeout(ctx, pullManifestTimeout)
	defer cancel()

	url := strings.TrimRight(peerURL, "/") + "/v1/weight/manifest/" + version
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return Manifest{}, fmt.Errorf("build req: %w", err)
	}
	req.Header.Set("User-Agent", "flowork-kernel-mesh/0.1")
	if key := apiKey(); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Manifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap manifest
	if err != nil {
		return Manifest{}, fmt.Errorf("read body: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse: %w", err)
	}
	return m, nil
}

// fetchChunk GET /v1/weight/chunk/{hash}. Limit body to 2x ChunkSize defensively.
func fetchChunk(ctx context.Context, peerURL, hash string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, pullChunkTimeout)
	defer cancel()

	url := strings.TrimRight(peerURL, "/") + "/v1/weight/chunk/" + hash
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build req: %w", err)
	}
	req.Header.Set("User-Agent", "flowork-kernel-mesh/0.1")
	if key := apiKey(); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Cap 2x ChunkSize (defensive — peer mengirim oversized = malicious).
	bodyCap := int64(ChunkSize) * 2
	body, err := io.ReadAll(io.LimitReader(resp.Body, bodyCap))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) >= bodyCap {
		return nil, fmt.Errorf("chunk oversized > %d bytes", bodyCap)
	}
	return body, nil
}
