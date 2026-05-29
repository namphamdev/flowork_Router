// Package mesh — kernel/mesh/peer_discover.go
//
// Peer discovery: list peer URL yang available untuk connect. MVP source
// of truth = settings DB key `MESH_PEER_SEEDS` (JSON array URL string).
//
// Pattern: bootstrap seed list user-controlled (Ayah set di Settings tab),
// kernel cuma read + try connect. Discovery dinamis (DHT Kademlia per
// Gemini red team finding) defer ke Phase J post-launch.
//
// Format settings DB key MESH_PEER_SEEDS:
//
//	["https://node-jakarta.flowork.ayah:3105", "https://vps-sg.flowork.ayah:3105"]
//
// Empty/missing → return empty list (single-node mode, no mesh peers).
//
// Plug-and-play: tambah peer = update settings DB, ngga butuh restart kernel.
// Cache 30s untuk reduce DB hit per discovery call.

package mesh

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/flowork/kernel/kernel/settings"
)

// Peer represent satu remote kernel node.
type Peer struct {
	// URL base HTTP endpoint (e.g., "https://node-x.flowork:3105").
	URL string `json:"url"`

	// Tags optional metadata (region, role, dll). Empty kalau ngga di-set.
	Tags []string `json:"tags,omitempty"`

	// PeerID — populated post-handshake (kernel/mesh/identity_handshake.go).
	// Format: flowork-peer://<prefix>:<uuid>. Empty kalau handshake belum
	// pernah berhasil.
	PeerID string `json:"peer_id,omitempty"`

	// PublicKey base64 ed25519 — populated post-handshake. Untuk verify
	// signature pada signed mesh sync, tool manifests, weight updates.
	PublicKey string `json:"public_key,omitempty"`

	// LastSeen timestamp (unix sec UTC) — last successful handshake/sync.
	LastSeen int64 `json:"last_seen,omitempty"`
}

const (
	// SeedsCacheTTL kapan cache di-refresh dari settings DB.
	SeedsCacheTTL = 30 * time.Second
)

var (
	cacheMu      sync.RWMutex
	cachedPeers  []Peer
	cachedAt     time.Time
)

// Discover return list peer dari settings DB seed config. Cached 30s.
//
// Empty list = single-node mode (kernel jalan tanpa peer mesh) — bukan
// error condition. Caller fallback ke standalone behavior.
//
// Errors:
//   - settings DB unavailable → return empty + log warning (graceful)
//   - JSON parse fail → return error (config malformed)
func Discover() ([]Peer, error) {
	cacheMu.RLock()
	if time.Since(cachedAt) < SeedsCacheTTL && cachedPeers != nil {
		out := append([]Peer{}, cachedPeers...)
		cacheMu.RUnlock()
		return out, nil
	}
	cacheMu.RUnlock()

	store := settings.Shared()
	if store == nil {
		// Settings unavailable — single-node mode, ngga error.
		setCache(nil)
		return nil, nil
	}

	raw, _ := store.Get("MESH_PEER_SEEDS")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		setCache(nil)
		return nil, nil
	}

	var peers []Peer
	if err := json.Unmarshal([]byte(raw), &peers); err != nil {
		// Try fallback: simple comma-separated URL list.
		urls := strings.Split(raw, ",")
		peers = make([]Peer, 0, len(urls))
		for _, u := range urls {
			u = strings.TrimSpace(u)
			if u != "" {
				peers = append(peers, Peer{URL: u})
			}
		}
		if len(peers) == 0 {
			return nil, fmt.Errorf("mesh.Discover: parse MESH_PEER_SEEDS: %w", err)
		}
	}

	// Sanitize: drop empty URL + dedupe by URL.
	seen := make(map[string]bool, len(peers))
	clean := make([]Peer, 0, len(peers))
	for _, p := range peers {
		p.URL = strings.TrimRight(strings.TrimSpace(p.URL), "/")
		if p.URL == "" || seen[p.URL] {
			continue
		}
		seen[p.URL] = true
		clean = append(clean, p)
	}

	setCache(clean)
	return clean, nil
}

// ResetCache clear cached peer list — caller pakai saat settings DB berubah
// dan mau langsung re-fetch (mis. setelah Ayah edit MESH_PEER_SEEDS via GUI).
func ResetCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachedPeers = nil
	cachedAt = time.Time{}
}

// setCache internal — write cache under lock.
func setCache(peers []Peer) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachedPeers = peers
	cachedAt = time.Now()
}
