// Package mesh — kernel/mesh/crdt_push.go
//
// Client-side CRDT push: edge node (Node B di belakang NAT) proaktif POST
// /v1/p2p/crdt/sync ke heavy peer (Node A public IP). Pattern asimetris
// untuk personal mode kalau Node B ngga punya public IP.
//
// Auth: Bearer token KERNEL_API_KEY (settings DB) — same key shared antar
// node. Whitelist /v1/mesh/info exempt karena handshake ngga butuh secret.
//
// Wire ke heartbeat: setelah ConnectAll, kalau peer alive, push CRDT.
// State per-peer last-known HLC tracked di lastSyncHLC map.

package mesh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/flowork/kernel/kernel/settings"
)

const (
	crdtPushTimeout = 10 * time.Second
)

// lastSyncHLC tracks HLC terakhir kita push/pull per-peer URL. Memory-only
// (Phase F2 mungkin persist ke settings DB kalau scale demand).
var (
	lastSyncMu  sync.RWMutex
	lastSyncHLC = make(map[string]HLC) // peerURL → last HLC
)

// PushCRDTToPeer push local events sejak last sync ke peer. Apply returned
// events. Update last-known HLC.
//
// Errors:
//   - peer URL empty / event log unavailable
//   - HTTP fail (network, 401 unauthorized, 5xx)
//   - JSON parse fail
//
// Best-effort — caller (heartbeat) ignore error, retry next tick.
func PushCRDTToPeer(ctx context.Context, peerURL string) error {
	if peerURL == "" {
		return fmt.Errorf("PushCRDTToPeer: peerURL empty")
	}

	log := SharedEventLog()
	if log == nil {
		return fmt.Errorf("PushCRDTToPeer: event log: %w", SharedEventLogErr())
	}

	// Read last-known HLC for this peer.
	lastSyncMu.RLock()
	since := lastSyncHLC[peerURL]
	lastSyncMu.RUnlock()

	// Get local events since last sync.
	events, err := log.ReadSince(since)
	if err != nil {
		return fmt.Errorf("PushCRDTToPeer: read events: %w", err)
	}

	// Build sync request.
	req := SyncRequest{
		SinceHLC: since,
		Events:   events,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("PushCRDTToPeer: marshal: %w", err)
	}

	// HTTP POST dengan Bearer token.
	cctx, cancel := context.WithTimeout(ctx, crdtPushTimeout)
	defer cancel()
	url := strings.TrimRight(peerURL, "/") + "/v1/p2p/crdt/sync"
	httpReq, err := http.NewRequestWithContext(cctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("PushCRDTToPeer: build req: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "flowork-kernel-mesh-crdt/0.1")
	if key := apiKey(); key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("PushCRDTToPeer: HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs := make([]byte, 512)
		n, _ := resp.Body.Read(bs)
		return fmt.Errorf("PushCRDTToPeer: HTTP %d: %s", resp.StatusCode, string(bs[:n]))
	}

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("PushCRDTToPeer: read resp: %w", err)
	}

	var syncResp SyncResponse
	if err := json.Unmarshal(rawBody, &syncResp); err != nil {
		return fmt.Errorf("PushCRDTToPeer: parse resp: %w", err)
	}

	// Apply returned events ke local log.
	if len(syncResp.Returned) > 0 {
		_, err := log.Apply(syncResp.Returned)
		if err != nil {
			return fmt.Errorf("PushCRDTToPeer: apply returned: %w", err)
		}
	}

	// Update last-known HLC = max HLC dari pushed + returned events.
	maxHLC := since
	for _, ev := range events {
		if ev.HLC.Compare(maxHLC) > 0 {
			maxHLC = ev.HLC
		}
	}
	for _, ev := range syncResp.Returned {
		if ev.HLC.Compare(maxHLC) > 0 {
			maxHLC = ev.HLC
		}
	}
	lastSyncMu.Lock()
	lastSyncHLC[peerURL] = maxHLC
	lastSyncMu.Unlock()

	return nil
}

// apiKey resolve KERNEL_API_KEY dari settings DB. Empty → no Bearer header.
func apiKey() string {
	store := settings.Shared()
	if store == nil {
		return ""
	}
	v, _ := store.Get("KERNEL_API_KEY")
	return strings.TrimSpace(v)
}
