// Package mesh — kernel/mesh/weight_seed.go
//
// Weight seeding side: serve manifest + chunk dari local storage. Heavy
// node yang udah complete download brain V4 register sebagai seeder, peer
// lain (edge / fresh node) Pull dari seeder.
//
// Storage layout (per kernel/path/.DataDir() → <data>/weights/):
//
//	<data>/weights/<version>/
//	    manifest.json        — Manifest struct serialized
//	    chunks/<hash>        — encrypted chunk file (4 MiB max)
//
// Endpoint (di kernel/api/route_weight.go):
//   GET /v1/weight/manifest/{version}  — return Manifest JSON
//   GET /v1/weight/chunk/{hash}        — return chunk bytes (encrypted)
//
// Per-chunk hash + Merkle root verify di-handle pull side. Seed side cuma
// serve apa yang ada di local storage. Caller harus trust seed... TIDAK,
// tetap pull side verify Merkle root + per-chunk hash anti chunk poisoning.

package mesh

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	kpath "github.com/flowork/kernel/kernel/path"
)

const (
	// ChunkSize default per docs/06-NETWORK.md § Torrent Mechanism.
	ChunkSize = 4 << 20 // 4 MiB

	// ManifestFilename canonical name di per-version dir.
	ManifestFilename = "manifest.json"
)

// Manifest schema spec opus-3:
//
//	{version, chunks[], hashes[], merkle_root, total_size, signature}
//
// Caller (pull side) verify per-chunk hash + final Merkle root. Signature
// optional Phase E2 — Phase J (license-bound) untuk full anti-tampering.
type Manifest struct {
	// Version model identifier, e.g., "brain_v4_q4".
	Version string `json:"version"`

	// Chunks ordered list of chunk metadata.
	Chunks []ChunkRef `json:"chunks"`

	// MerkleRoot hex-encoded root hash dari binary Merkle tree atas
	// chunk hashes. Caller verify final.
	MerkleRoot string `json:"merkle_root"`

	// TotalSize bytes (sum of chunks).
	TotalSize int64 `json:"total_size"`

	// Signature placeholder — Phase J license-bound signature
	// (Ed25519 signed by Ayah's private key). Empty di Phase E2.
	Signature string `json:"signature,omitempty"`

	// EncryptedKey AES-256 key encrypted dengan license public key.
	// Pull side decrypt via license.DecryptKey(EncryptedKey) → AES key
	// untuk decrypt chunk bytes. Empty di MVP (no encryption pre-license).
	EncryptedKey string `json:"encrypted_key,omitempty"`
}

// ChunkRef entry di Manifest.Chunks.
type ChunkRef struct {
	// Hash sha256 hex of chunk plaintext bytes (or encrypted, see Mode).
	Hash string `json:"hash"`

	// Size bytes (last chunk bisa lebih kecil dari ChunkSize).
	Size int64 `json:"size"`

	// Index urutan chunk (0-based, untuk reconstruct sequential).
	Index int `json:"index"`
}

// LoadLocalManifest baca manifest.json dari local weight dir untuk version.
// Pulled chunks belum verified — caller (Pull) yang Merkle-verify.
func LoadLocalManifest(version string) (Manifest, error) {
	if !validVersion(version) {
		return Manifest{}, fmt.Errorf("LoadLocalManifest: invalid version %q", version)
	}
	dir, err := versionDir(version)
	if err != nil {
		return Manifest{}, err
	}
	manifestPath := filepath.Join(dir, ManifestFilename)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, fmt.Errorf("LoadLocalManifest: %s not seeded yet", version)
		}
		return Manifest{}, fmt.Errorf("LoadLocalManifest: read: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("LoadLocalManifest: parse: %w", err)
	}
	return m, nil
}

// SaveLocalManifest write manifest.json ke local weight dir. Atomic via
// tmp+rename. Caller responsible untuk chunk file write (separate flow).
func SaveLocalManifest(m Manifest) error {
	if !validVersion(m.Version) {
		return fmt.Errorf("SaveLocalManifest: invalid version %q", m.Version)
	}
	dir, err := versionDir(m.Version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("SaveLocalManifest: mkdir: %w", err)
	}

	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("SaveLocalManifest: marshal: %w", err)
	}

	tmp := filepath.Join(dir, ManifestFilename+".tmp")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("SaveLocalManifest: write tmp: %w", err)
	}
	final := filepath.Join(dir, ManifestFilename)
	return os.Rename(tmp, final)
}

// ServeChunk read chunk file by hash. Return io.ReadCloser yang caller
// WAJIB Close(). Hash validation: ngga match chunk file actual content
// = error (silent corruption guard).
//
// Caller (HTTP handler) stream langsung ke ResponseWriter via io.Copy.
func ServeChunk(version, hash string) (io.ReadCloser, int64, error) {
	if !validVersion(version) {
		return nil, 0, fmt.Errorf("ServeChunk: invalid version")
	}
	if !validChunkHash(hash) {
		return nil, 0, fmt.Errorf("ServeChunk: invalid hash format")
	}
	dir, err := versionDir(version)
	if err != nil {
		return nil, 0, err
	}
	chunkPath := filepath.Join(dir, "chunks", hash)
	f, err := os.Open(chunkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, fmt.Errorf("ServeChunk: not found %s/%s", version, hash)
		}
		return nil, 0, fmt.Errorf("ServeChunk: open: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("ServeChunk: stat: %w", err)
	}
	return f, st.Size(), nil
}

// SaveChunk write chunk file ke local storage (used by pull side after
// hash verify, or by initial seed setup). Atomic via tmp+rename.
//
// Verifies hash match content sebelum write — anti silent corruption.
func SaveChunk(version, hash string, data []byte) error {
	if !validVersion(version) {
		return fmt.Errorf("SaveChunk: invalid version")
	}
	if !validChunkHash(hash) {
		return fmt.Errorf("SaveChunk: invalid hash format")
	}
	// Verify content matches claimed hash.
	sum := sha256.Sum256(data)
	gotHash := hex.EncodeToString(sum[:])
	if gotHash != hash {
		return fmt.Errorf("SaveChunk: hash mismatch (claimed %s, got %s)", hash, gotHash)
	}

	dir, err := versionDir(version)
	if err != nil {
		return err
	}
	chunksDir := filepath.Join(dir, "chunks")
	if err := os.MkdirAll(chunksDir, 0o755); err != nil {
		return fmt.Errorf("SaveChunk: mkdir: %w", err)
	}

	tmp := filepath.Join(chunksDir, hash+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("SaveChunk: write tmp: %w", err)
	}
	final := filepath.Join(chunksDir, hash)
	return os.Rename(tmp, final)
}

// saveChunkRaw write bytes ke chunk file TANPA hash verify. Internal
// helper untuk Pull-with-decrypt path — hash verified atas ciphertext
// pre-decrypt, plaintext doesn't need to match. Filename pakai claimed
// hash (dari manifest, ciphertext sha256) supaya ServeChunk lookup OK.
func saveChunkRaw(version, hash string, data []byte) error {
	if !validVersion(version) {
		return fmt.Errorf("saveChunkRaw: invalid version")
	}
	if !validChunkHash(hash) {
		return fmt.Errorf("saveChunkRaw: invalid hash format")
	}
	dir, err := versionDir(version)
	if err != nil {
		return err
	}
	chunksDir := filepath.Join(dir, "chunks")
	if err := os.MkdirAll(chunksDir, 0o755); err != nil {
		return fmt.Errorf("saveChunkRaw: mkdir: %w", err)
	}
	tmp := filepath.Join(chunksDir, hash+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("saveChunkRaw: write tmp: %w", err)
	}
	final := filepath.Join(chunksDir, hash)
	return os.Rename(tmp, final)
}

// HasLocalVersion check apakah local seeder punya complete weight set untuk
// version (manifest + semua chunk hash exist). Untuk seeder advertisement.
func HasLocalVersion(version string) bool {
	m, err := LoadLocalManifest(version)
	if err != nil {
		return false
	}
	dir, err := versionDir(version)
	if err != nil {
		return false
	}
	chunksDir := filepath.Join(dir, "chunks")
	for _, c := range m.Chunks {
		if _, err := os.Stat(filepath.Join(chunksDir, c.Hash)); err != nil {
			return false
		}
	}
	return true
}

// BuildManifestFromChunks compute Manifest dari ordered list of chunk
// data (untuk seeder initial setup). Compute per-chunk hash + Merkle root.
//
// Caller pass plaintext chunks; encryption layer (Phase J license-bound)
// applied separately sebelum SaveChunk.
func BuildManifestFromChunks(version string, chunks [][]byte) (Manifest, error) {
	if !validVersion(version) {
		return Manifest{}, errors.New("BuildManifest: invalid version")
	}
	if len(chunks) == 0 {
		return Manifest{}, errors.New("BuildManifest: no chunks")
	}

	refs := make([]ChunkRef, 0, len(chunks))
	hashes := make([]string, 0, len(chunks))
	var totalSize int64
	for i, c := range chunks {
		sum := sha256.Sum256(c)
		h := hex.EncodeToString(sum[:])
		refs = append(refs, ChunkRef{
			Hash:  h,
			Size:  int64(len(c)),
			Index: i,
		})
		hashes = append(hashes, h)
		totalSize += int64(len(c))
	}

	root, err := MerkleRoot(hashes)
	if err != nil {
		return Manifest{}, fmt.Errorf("BuildManifest: merkle: %w", err)
	}

	return Manifest{
		Version:    version,
		Chunks:     refs,
		MerkleRoot: root,
		TotalSize:  totalSize,
	}, nil
}

// versionDir resolve <data>/weights/<version>/ — caller MkdirAll kalau write.
func versionDir(version string) (string, error) {
	dataDir, err := kpath.DataDir()
	if err != nil {
		return "", fmt.Errorf("versionDir: %w", err)
	}
	return filepath.Join(dataDir, "weights", version), nil
}

// validVersion sanitize: alphanumeric + underscore + dot + dash, 1-64 chars.
// Anti path traversal (no "..", "/", "\").
func validVersion(v string) bool {
	if len(v) < 1 || len(v) > 64 {
		return false
	}
	if strings.Contains(v, "..") || strings.ContainsAny(v, "/\\") {
		return false
	}
	for _, c := range v {
		ok := (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '-' || c == '.'
		if !ok {
			return false
		}
	}
	return true
}

// validChunkHash sha256 hex (64 hex chars).
func validChunkHash(h string) bool {
	if len(h) != 64 {
		return false
	}
	for _, c := range h {
		ok := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !ok {
			return false
		}
	}
	return true
}

// MerkleRoot compute root atas list of hex-encoded leaf hashes. Binary tree,
// odd-count duplicate last leaf (Bitcoin-style anti CVE-2012-2459 — caveat:
// MUST use unique chunk hashes which ours are by construction sha256 distinct).
//
// Empty input → error. Single leaf → return that leaf as root.
func MerkleRoot(leafHashes []string) (string, error) {
	if len(leafHashes) == 0 {
		return "", errors.New("MerkleRoot: empty leaves")
	}
	level := make([][]byte, 0, len(leafHashes))
	for _, h := range leafHashes {
		b, err := hex.DecodeString(h)
		if err != nil || len(b) != 32 {
			return "", fmt.Errorf("MerkleRoot: invalid leaf hash %q", h)
		}
		level = append(level, b)
	}

	for len(level) > 1 {
		next := make([][]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			var pair []byte
			if i+1 < len(level) {
				pair = append(level[i], level[i+1]...)
			} else {
				// Odd: duplicate last (Bitcoin convention).
				pair = append(level[i], level[i]...)
			}
			sum := sha256.Sum256(pair)
			next = append(next, sum[:])
		}
		level = next
	}

	return hex.EncodeToString(level[0]), nil
}
