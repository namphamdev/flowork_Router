// modellock.go — per-(provider, model) temporary cooldown lock.
//
// Go analogue of 9router's `modelLock_<model>` granularity: when a provider
// errors for a SPECIFIC model (429 / quota / 5xx / 401…), only that
// (provider, model) pair is parked for a cooldown window — other models on the
// same provider keep flowing, and the same model on other providers is
// unaffected. Cooldown + exponential backoff are computed by the existing
// proven 17-rule table in internal/services (CheckFallbackError), so this stays
// consistent with the router's documented fallback policy.
//
// In-memory + mutex-guarded (same pattern as roundRobinCursor in strategy.go):
// single-owner router, locks are transient by design, no need to persist.

package router

import (
	"sync"
	"time"

	"github.com/flowork-os/flowork_Router/internal/services"
	"github.com/flowork-os/flowork_Router/internal/store"
)

type modelLockEntry struct {
	until        time.Time
	backoffLevel int
}

var (
	modelLocks = map[string]modelLockEntry{} // key = providerID + "\x00" + model
	mlMu       sync.Mutex
)

func modelLockKey(providerID, model string) string {
	return providerID + "\x00" + model
}

// isModelLocked reports whether (providerID, model) is currently in cooldown.
// Expired entries are lazily deleted so the map doesn't grow unbounded.
func isModelLocked(providerID, model string) bool {
	mlMu.Lock()
	defer mlMu.Unlock()
	e, ok := modelLocks[modelLockKey(providerID, model)]
	if !ok {
		return false
	}
	if time.Now().After(e.until) {
		delete(modelLocks, modelLockKey(providerID, model))
		return false
	}
	return true
}

// lockModel parks (providerID, model) for a cooldown derived from the error.
// Backoff escalates per consecutive failure (carried in the entry). No-op when
// the rule table says the error shouldn't trigger a fallback/cooldown.
func lockModel(providerID, model string, status int, errText string) {
	mlMu.Lock()
	defer mlMu.Unlock()
	key := modelLockKey(providerID, model)
	prev := modelLocks[key].backoffLevel
	dec := services.CheckFallbackError(status, errText, prev)
	if !dec.ShouldFallback || dec.Cooldown <= 0 {
		return
	}
	modelLocks[key] = modelLockEntry{
		until:        time.Now().Add(dec.Cooldown),
		backoffLevel: dec.NewBackoffLevel,
	}
}

// clearModelLock removes the lock + resets backoff for (providerID, model) on a
// successful request, so a recovered model is immediately preferred again.
func clearModelLock(providerID, model string) {
	mlMu.Lock()
	defer mlMu.Unlock()
	delete(modelLocks, modelLockKey(providerID, model))
}

// reorderByModelLock moves currently-locked (provider, model) pairs to the BACK
// of the candidate list instead of dropping them — so a healthy provider is
// tried first, but a fully-locked model still gets a last-resort attempt rather
// than a hard 503 (zero regression vs the pre-lock behaviour). Order within each
// group is preserved (stable).
func reorderByModelLock(matches []store.ProviderConnection, model string) []store.ProviderConnection {
	if len(matches) < 2 {
		return matches
	}
	avail := make([]store.ProviderConnection, 0, len(matches))
	locked := make([]store.ProviderConnection, 0)
	for _, p := range matches {
		if isModelLocked(p.ID, model) {
			locked = append(locked, p)
		} else {
			avail = append(avail, p)
		}
	}
	if len(locked) == 0 {
		return matches
	}
	return append(avail, locked...)
}
