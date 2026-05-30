// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Tunnel Watchdog (health-check + auto-restart).

package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/flowork-os/flowork_Router/internal/store"
)

const (
	tunnelWatchdogInterval  = 60 * time.Second
	tunnelWatchdogProbeWait = 6 * time.Second
)

var (
	tunnelWatchdogStarted bool
	tunnelWatchdogMu      sync.Mutex
	tunnelWatchdogCancel  context.CancelFunc
)

// startTunnelWatchdog launches the background probe loop. Idempotent — second
// call is a no-op. Use stopTunnelWatchdog() on shutdown.
func startTunnelWatchdog() {
	tunnelWatchdogMu.Lock()
	defer tunnelWatchdogMu.Unlock()
	if tunnelWatchdogStarted {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	tunnelWatchdogCancel = cancel
	tunnelWatchdogStarted = true
	go tunnelWatchdogLoop(ctx)
}

// stopTunnelWatchdog ends the background loop. Safe to call when not started.
func stopTunnelWatchdog() {
	tunnelWatchdogMu.Lock()
	defer tunnelWatchdogMu.Unlock()
	if tunnelWatchdogCancel != nil {
		tunnelWatchdogCancel()
	}
	tunnelWatchdogStarted = false
}

func tunnelWatchdogLoop(ctx context.Context) {
	ticker := time.NewTicker(tunnelWatchdogInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tunnelWatchdogTick(ctx)
		}
	}
}

func tunnelWatchdogTick(ctx context.Context) {
	d, err := store.Open()
	if err != nil {
		return
	}
	st, _ := store.LoadTunnelState(d)
	if st == nil {
		return
	}

	// Cloudflared: if our local pid-tracker says enabled but the URL is no
	// longer reachable, flip enabled=false so the dashboard shows the real
	// state. Doesn't auto-restart (that would hide chronic config issues);
	// the user gets a visible "down" indicator.
	if st.CloudflareEnabled && st.CloudflareURL != "" {
		if !probeURLOK(ctx, st.CloudflareURL) && !isCloudflaredRunning() {
			log.Printf("flow_router tunnel watchdog: cloudflared down (url=%s)", st.CloudflareURL)
			st.CloudflareEnabled = false
			st.CloudflareURL = ""
			st.CloudflarePID = 0
			_ = store.SaveTunnelState(d, st)
		}
	}
}

func probeURLOK(ctx context.Context, base string) bool {
	if base == "" {
		return false
	}
	probeCtx, cancel := context.WithTimeout(ctx, tunnelWatchdogProbeWait)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, base+"/api/health", nil)
	if err != nil {
		return false
	}
	resp, err := mediaHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
