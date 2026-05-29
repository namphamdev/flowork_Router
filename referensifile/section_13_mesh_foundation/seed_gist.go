// Package mesh — seed_gist.go: GitHub Gist bootstrap fallback (M2).
//
// Per M02-mesh-discovery-mdns.md §Step 4:
// When mDNS alone isn't enough (WAN peers), kernel can pull a signed seed
// list from a public GitHub Gist. The gist content is signed by the master
// Ed25519 key to prevent tampering.
//
// Settings DB key: MESH_SEED_GIST_URL (default empty = skip gist bootstrap).
// When set, kernel fetches gist JSON, verifies master signature, and adds
// seed peers to the discovery pipeline.
package mesh

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/flowork/kernel/kernel/settings"
)

const (
	settingsKeySeedGistURL = "MESH_SEED_GIST_URL"
	gistFetchTimeout       = 10 * time.Second
	maxGistSize            = 1 << 20 // 1MB max
)

// GistSeed — one entry in the signed gist seed list.
type GistSeed struct {
	PubKeyHex string `json:"pubkey"` // ed25519 public key hex
	IP        string `json:"ip"`     // last-known IP
	Port      int    `json:"port"`   // kernel port (default 3105)
}

// SignedGistDoc — the full gist document with signature verification.
//
// Sign target = sha256(canonicalSeedsBytes(seeds, timestamp)). Per W-2 fix
// (post-Gemini-audit AMENDMENTS-V1), JANGAN sign json.Marshal output —
// non-deterministic key ordering bisa bikin signature false-reject di
// cross-platform (Go map ordering, OS encoding diff).
type SignedGistDoc struct {
	Seeds     []GistSeed `json:"seeds"`
	Signature string     `json:"signature"` // hex ed25519 sign(sha256(canonical bytes))
	Timestamp string     `json:"timestamp"` // ISO 8601 — included in signed canonical
}

// canonicalSeedsBytes — deterministic byte encoding untuk seed list + timestamp.
// Format (each field separator 0x00):
//
//	timestamp || 0x00 ||
//	count_be32 || 0x00 ||
//	for each seed (sorted by pubkey hex asc):
//	  pubkey_hex || 0x00 || ip || 0x00 || port_be32 || 0x00
//
// Sort by pubkey memastikan order deterministic di Go map iteration.
func canonicalSeedsBytes(seeds []GistSeed, timestamp string) []byte {
	// Sort copy — jangan mutate input slice
	sorted := make([]GistSeed, len(seeds))
	copy(sorted, seeds)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PubKeyHex < sorted[j].PubKeyHex
	})

	var buf []byte
	buf = append(buf, []byte(timestamp)...)
	buf = append(buf, 0x00)
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(sorted)))
	buf = append(buf, n[:]...)
	buf = append(buf, 0x00)
	for _, s := range sorted {
		buf = append(buf, []byte(s.PubKeyHex)...)
		buf = append(buf, 0x00)
		buf = append(buf, []byte(s.IP)...)
		buf = append(buf, 0x00)
		var p [4]byte
		binary.BigEndian.PutUint32(p[:], uint32(s.Port))
		buf = append(buf, p[:]...)
		buf = append(buf, 0x00)
	}
	return buf
}

// PullGistSeeds fetches and verifies the seed list from a GitHub Gist.
//
// Returns nil, nil if gist URL is not configured (skip mode).
// Returns error if fetch fails or signature is invalid.
//
// INVARIANT 2: filters out cloud metadata IPs before returning.
func PullGistSeeds(ctx context.Context, gistRawURL string, masterPubKey ed25519.PublicKey) ([]GistSeed, error) {
	if strings.TrimSpace(gistRawURL) == "" {
		return nil, nil // skip — mDNS-only mode
	}

	// Fetch gist content
	fetchCtx, cancel := context.WithTimeout(ctx, gistFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, "GET", gistRawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gist seed: create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gist seed: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gist seed: HTTP %d", resp.StatusCode)
	}

	// Read with size limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxGistSize))
	if err != nil {
		return nil, fmt.Errorf("gist seed: read body: %w", err)
	}

	// Parse
	var doc SignedGistDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("gist seed: parse JSON: %w", err)
	}

	// Verify signature — canonical bytes hash-then-sign (W-2 fix per AMENDMENTS-V1).
	// JANGAN sign json.Marshal output (non-deterministic key order across Go versions / OS).
	if masterPubKey != nil {
		canonical := canonicalSeedsBytes(doc.Seeds, doc.Timestamp)
		hash := sha256.Sum256(canonical)
		sig, err := hex.DecodeString(doc.Signature)
		if err != nil {
			return nil, fmt.Errorf("gist seed: decode signature: %w", err)
		}
		if !ed25519.Verify(masterPubKey, hash[:], sig) {
			return nil, errors.New("gist seed: signature invalid (master key mismatch or canonical bytes tampered)")
		}
	}

	// Filter cloud metadata IPs (INVARIANT 2)
	var safe []GistSeed
	for _, s := range doc.Seeds {
		if IsCloudMetadataIP(s.IP) {
			log.Printf("[gist-seed] REJECT seed %s@%s: cloud metadata IP (INVARIANT 2)",
				s.PubKeyHex[:8], s.IP)
			continue
		}
		safe = append(safe, s)
	}

	log.Printf("[gist-seed] loaded %d seeds from gist (%d rejected by INVARIANT 2)",
		len(safe), len(doc.Seeds)-len(safe))

	return safe, nil
}

// PullGistSeedsFromSettings convenience wrapper — reads gist URL from
// settings DB and calls PullGistSeeds.
func PullGistSeedsFromSettings(ctx context.Context, masterPubKey ed25519.PublicKey) ([]GistSeed, error) {
	store := settings.Shared()
	if store == nil {
		return nil, nil
	}
	url, _ := store.Get(settingsKeySeedGistURL)
	return PullGistSeeds(ctx, url, masterPubKey)
}
