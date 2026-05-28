// Antigravity session ID cache.
//
// Antigravity's CloudCode endpoint uses an X-Machine-Session-Id header to
// scope its prompt cache. The native binary generates ONE session id at
// startup (`uuidv4() + Date.now()`) and keeps it for the process lifetime,
// scoped per connection. flow_router is long-running, so we simulate
// per-launch behaviour: stable id per connectionId within a single binary
// run, but a fresh one when the router restarts.
//
// Without this, every Antigravity request looks like a new session →
// prompt cache misses → significantly higher per-request cost.

package executors

import (
	"crypto/rand"
	"strconv"
	"sync"
	"time"
)

const (
	// sessionTTL — entries unused for this long are evicted.
	sessionTTL = 2 * time.Hour
	// sessionCleanupInterval — how often the eviction sweep runs.
	sessionCleanupInterval = 30 * time.Minute
	// sessionMaxEntries — safety cap between sweeps so a buggy caller
	// flooding fresh ids can't OOM the process.
	sessionMaxEntries = 1000
)

type sessionEntry struct {
	id       string
	lastUsed time.Time
}

var (
	sessionMu    sync.Mutex
	sessionStore = map[string]*sessionEntry{}
	sessionInit  sync.Once
)

// startSessionSweeper launches the eviction goroutine exactly once.
func startSessionSweeper() {
	sessionInit.Do(func() {
		go func() {
			t := time.NewTicker(sessionCleanupInterval)
			defer t.Stop()
			for range t.C {
				sessionMu.Lock()
				now := time.Now()
				for k, e := range sessionStore {
					if now.Sub(e.lastUsed) > sessionTTL {
						delete(sessionStore, k)
					}
				}
				sessionMu.Unlock()
			}
		}()
	})
}

// DeriveAntigravitySessionID returns a stable session id for connectionID.
// First call per connection mints a new id; subsequent calls return the same
// value (with last-used touched for TTL). An empty connectionID always mints
// a one-off id (never cached).
func DeriveAntigravitySessionID(connectionID string) string {
	startSessionSweeper()
	if connectionID == "" {
		return GenerateAntigravitySessionID()
	}
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if e, ok := sessionStore[connectionID]; ok {
		e.lastUsed = time.Now()
		return e.id
	}
	// Safety cap: drop the oldest entry before we exceed the max.
	if len(sessionStore) >= sessionMaxEntries {
		var oldestKey string
		var oldestTime time.Time
		for k, e := range sessionStore {
			if oldestKey == "" || e.lastUsed.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.lastUsed
			}
		}
		delete(sessionStore, oldestKey)
	}
	id := GenerateAntigravitySessionID()
	sessionStore[connectionID] = &sessionEntry{id: id, lastUsed: time.Now()}
	return id
}

// GenerateAntigravitySessionID mints an id in the native binary's exact
// format: `<uuid_v4><millis_since_epoch>` (no separator).
func GenerateAntigravitySessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0F) | 0x40 // RFC 4122 version 4
	b[8] = (b[8] & 0x3F) | 0x80 // variant
	const hexd = "0123456789abcdef"
	out := make([]byte, 36)
	bi := 0
	for i := 0; i < 36; i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			out[i] = '-'
			continue
		}
		out[i] = hexd[b[bi]>>4]
		i++
		out[i] = hexd[b[bi]&0xF]
		bi++
	}
	return string(out) + strconv.FormatInt(time.Now().UnixMilli(), 10)
}

// ClearAntigravitySessionStore wipes every cached id. Useful for tests and
// for explicit "rotate all sessions" admin actions.
func ClearAntigravitySessionStore() {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	sessionStore = map[string]*sessionEntry{}
}

// AntigravitySessionStoreSize returns the current entry count (test helper).
func AntigravitySessionStoreSize() int {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	return len(sessionStore)
}

