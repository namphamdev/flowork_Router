// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Token Refresh Worker (background OAuth refresh).

package services

import (
	"context"
	"log"
	"sync"
	"time"
)

// RefreshLead is the default lead time used when a provider doesn't appear
// in RefreshLeadByProvider. Long-lived OAuth tokens (e.g. Claude consumer
// OAuth at ~4h) get more lead than short-lived device tokens (qwen ~20m).
var RefreshLead = 5 * time.Minute

// RefreshLeadByProvider lets per-provider token lifetimes drive their own
// refresh cadence. Keys are normalised lower-case provider identifiers as
// returned by TokenSource.Provider(). Unknown providers fall back to the
// package-level RefreshLead constant.
var RefreshLeadByProvider = map[string]time.Duration{
	"codex":       5 * 24 * time.Hour, // 5 days — Codex tokens last ~30d
	"openai":      5 * 24 * time.Hour, // alias
	"claude":      4 * time.Hour,      // Claude consumer OAuth ~12h
	"anthropic":   4 * time.Hour,      // alias
	"iflow":       24 * time.Hour,     // iFlow tokens ~14 days
	"qwen":        20 * time.Minute,   // Qwen device tokens ~1h
	"kimi-coding": 5 * time.Minute,    // Kimi tokens ~30m
	"kimi":        5 * time.Minute,
	"antigravity": 5 * time.Minute, // Google CloudCode tokens ~1h
	"gemini-cli":  5 * time.Minute, // alias
	"github":      4 * time.Hour,   // Copilot tokens ~12h
	"copilot":     4 * time.Hour,   // alias
	"kiro":        4 * time.Hour,   // AWS SSO tokens ~8h
}

// leadFor returns the refresh lead time for a provider, falling back to the
// package-level RefreshLead when the provider is unknown.
func leadFor(provider string) time.Duration {
	if d, ok := RefreshLeadByProvider[provider]; ok && d > 0 {
		return d
	}
	return RefreshLead
}

// FailureRetry is the polling interval when a refresh failed.
var FailureRetry = 60 * time.Second

// TokenSource is what the caller provides per provider: a way to read the
// current expiry and to perform the refresh. flow_router uses it from
// internal/store/oauth tokens. Refresh returns the new expiry.
type TokenSource interface {
	Provider() string
	ExpiresAt() time.Time
	Refresh(ctx context.Context) (time.Time, error)
}

// Worker polls TokenSources and refreshes each just before expiry.
type Worker struct {
	mu      sync.Mutex
	sources []TokenSource
	cancel  context.CancelFunc
	started bool
	wg      sync.WaitGroup // Stop() waits on this so the loop actually exited
}

// NewWorker returns an empty worker. Add() to register sources, then Start().
func NewWorker() *Worker { return &Worker{} }

// Add registers a TokenSource. Safe to call before or after Start.
func (w *Worker) Add(src TokenSource) {
	w.mu.Lock()
	w.sources = append(w.sources, src)
	w.mu.Unlock()
}

// Start launches the background refresh loop. Idempotent.
func (w *Worker) Start() {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.started = true
	w.wg.Add(1)
	w.mu.Unlock()
	go func() {
		defer w.wg.Done()
		w.loop(ctx)
	}()
}

// Stop ends the worker AND waits for the background goroutine to exit, so
// callers can safely tear down shared state after returning.
func (w *Worker) Stop() {
	w.mu.Lock()
	if w.cancel != nil {
		w.cancel()
	}
	w.started = false
	w.mu.Unlock()
	w.wg.Wait()
}

func (w *Worker) loop(ctx context.Context) {
	for {
		w.mu.Lock()
		sources := append([]TokenSource(nil), w.sources...)
		w.mu.Unlock()

		// Find the soonest source needing refresh
		now := time.Now()
		var nextWake time.Duration = FailureRetry
		var due TokenSource
		for _, s := range sources {
			exp := s.ExpiresAt()
			if exp.IsZero() {
				continue // no expiry recorded yet → leave alone
			}
			refreshAt := exp.Add(-leadFor(s.Provider()))
			if !refreshAt.After(now) {
				// already due → handle this one first
				due = s
				nextWake = 0
				break
			}
			wait := time.Until(refreshAt)
			if wait < nextWake {
				nextWake = wait
			}
		}

		if due != nil {
			if newExp, err := due.Refresh(ctx); err != nil {
				log.Printf("flow_router token refresh failed for %s: %v", due.Provider(), err)
				nextWake = FailureRetry
			} else {
				log.Printf("flow_router token refreshed for %s; next expiry %s", due.Provider(), newExp.Format(time.RFC3339))
				continue // re-evaluate immediately, another source may also be due
			}
		}

		// No source registered, or nothing due — wait nextWake (or 1h cap)
		if nextWake <= 0 {
			nextWake = time.Hour
		}
		if nextWake > time.Hour {
			nextWake = time.Hour
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(nextWake):
		}
	}
}
