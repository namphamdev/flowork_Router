// Token Refresh Worker (background OAuth refresh).

package services

import (
	"context"
	"log"
	"sync"
	"time"
)

// RefreshLead is how far ahead of expiry we attempt a refresh.
var RefreshLead = 5 * time.Minute

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
			refreshAt := exp.Add(-RefreshLead)
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
