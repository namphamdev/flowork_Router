// Package mesh — kernel/mesh/heartbeat_peer.go
//
// Periodic peer heartbeat loop. Cek peer alive via Connect(), update
// PeerStatus map, expose ke API + GUI status panel.
//
// Pattern: caller spawn StartHeartbeat goroutine di kernel boot
// (cmd/kernel/main.go). Tick interval 5min default — config via settings DB
// MESH_HEARTBEAT_SEC. Default lebih panjang biar irit network/baterai untuk
// personal mode multi-komputer. Kalau perlu sync cepat (high-traffic),
// set MESH_HEARTBEAT_SEC=60 di settings.
//
// Stale threshold: 3x tick interval = peer considered dead → drop dari
// active list, retry connect at next tick.

package mesh

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flowork/kernel/kernel/settings"
)

const (
	defaultHeartbeatTick = 5 * time.Minute
	staleMultiplier      = 3 // peer dead kalau no heartbeat > tick * multiplier
)

// PeerStatus snapshot per peer — last successful contact + latency.
type PeerStatus struct {
	URL          string    `json:"url"`
	Alive        bool      `json:"alive"`
	LastSeen     time.Time `json:"last_seen,omitempty"`
	LatencyMs    int       `json:"latency_ms,omitempty"`
	NodeID       string    `json:"node_id,omitempty"`
	Version      string    `json:"version,omitempty"`
	WargaCount   int       `json:"warga_count,omitempty"`
	ToolCount    int       `json:"tool_count,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
	ConsecutiveFails int   `json:"consecutive_fails,omitempty"`
}

var (
	statusMu sync.RWMutex
	statuses = make(map[string]*PeerStatus)
)

// StartHeartbeat blocking loop — caller wrap di goroutine. Stop via ctx cancel.
//
func StartHeartbeat(ctx context.Context) {
	tick := heartbeatTick()
	peers, err := Discover()
	switch {
	case err != nil:
		log.Printf("mesh: peer discover error: %v — single-node mode", err)
	case len(peers) == 0:
		log.Printf("mesh: no peers configured (MESH_PEER_SEEDS empty) — single-node mode. " +
			"To add peers, set MESH_PEER_SEEDS in settings DB or via GUI Mesh tab.")
	default:
		urls := make([]string, len(peers))
		for i, p := range peers {
			urls[i] = p.URL
		}
		log.Printf("mesh: heartbeat started — tick=%s, %d peer(s): %s",
			tick, len(peers), strings.Join(urls, ", "))
	}

	for {
		tick = heartbeatTick()
		select {
		case <-ctx.Done():
			return
		case <-time.After(tick):
		}

		runHeartbeatOnce(ctx)
	}
}

// runHeartbeatOnce satu round full heartbeat — connect ke semua peer,
// update status, push CRDT events asimetris (edge → heavy peer).
// Caller (StartHeartbeat) loop atas ini.
func runHeartbeatOnce(ctx context.Context) {
	infos, errs := ConnectAll(ctx)

	// Refresh status alive per info.
	now := time.Now().UTC()
	aliveURLs := make([]string, 0, len(infos))
	statusMu.Lock()
	for _, info := range infos {
		statuses[info.URL] = &PeerStatus{
			URL:        info.URL,
			Alive:      true,
			LastSeen:   now,
			LatencyMs:  info.LatencyMs,
			NodeID:     info.NodeID,
			Version:    info.Version,
			WargaCount: info.WargaCount,
			ToolCount:  info.ToolCount,
		}
		aliveURLs = append(aliveURLs, info.URL)
	}
	statusMu.Unlock()

	// CRDT push best-effort ke peer alive (Phase E2-A2 asimetris pattern).
	// Edge node (di belakang NAT) push events + receive merged state via
	// /v1/p2p/crdt/sync. Heavy peer pasif accept. Error → log ke peer
	// status, retry next tick.
	for _, url := range aliveURLs {
		if err := PushCRDTToPeer(ctx, url); err != nil {
			statusMu.Lock()
			if st, ok := statuses[url]; ok {
				st.LastError = "crdt push: " + err.Error()
			}
			statusMu.Unlock()
		}
	}

	statusMu.Lock()
	defer statusMu.Unlock()

	// Mark errored peer (parse URL from err msg lossy — better: pass through).
	// Iterate Discover() results untuk catch peer yang ngga di infos.
	peers, _ := Discover()
	for _, p := range peers {
		if _, ok := statuses[p.URL]; ok && statuses[p.URL].Alive && statuses[p.URL].LastSeen.Equal(now) {
			continue
		}
		// Peer ngga ada di infos → set alive=false (mungkin connect fail).
		st, exists := statuses[p.URL]
		if !exists {
			st = &PeerStatus{URL: p.URL}
			statuses[p.URL] = st
		}
		st.Alive = false
		st.ConsecutiveFails++
		// Bind first matching err.
		for _, e := range errs {
			if e != nil {
				st.LastError = e.Error()
				break
			}
		}
	}

	// Stale eviction: peer LastSeen > tick * staleMultiplier ago → mark dead.
	tick := heartbeatTick()
	threshold := time.Duration(staleMultiplier) * tick
	for _, st := range statuses {
		if !st.LastSeen.IsZero() && now.Sub(st.LastSeen) > threshold {
			st.Alive = false
		}
	}
}

// Statuses snapshot semua peer — untuk API /v1/mesh/peers + GUI panel.
func Statuses() []PeerStatus {
	statusMu.RLock()
	defer statusMu.RUnlock()
	out := make([]PeerStatus, 0, len(statuses))
	for _, st := range statuses {
		// Defensive copy (caller modify ngga ngotorin internal).
		copy := *st
		out = append(out, copy)
	}
	return out
}

// heartbeatTick read interval dari settings DB. Fallback default 60s.
func heartbeatTick() time.Duration {
	if store := settings.Shared(); store != nil {
		if v, _ := store.Get("MESH_HEARTBEAT_SEC"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 3600 {
				return time.Duration(n) * time.Second
			}
		}
	}
	return defaultHeartbeatTick
}
