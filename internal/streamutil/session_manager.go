// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Session Manager (per-connection ID for prompt cache).

package streamutil

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	sessionTTL             = 12 * time.Hour
	sessionCleanupInterval = 30 * time.Minute
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

func startSessionCleanup() {
	go func() {
		ticker := time.NewTicker(sessionCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-sessionTTL)
			sessionMu.Lock()
			for k, e := range sessionStore {
				if e.lastUsed.Before(cutoff) {
					delete(sessionStore, k)
				}
			}
			sessionMu.Unlock()
		}
	}()
}

// DeriveSessionID returns a stable session ID for connectionID, generating
// one on first call. Re-use within the process keeps prompt-cache hot.
func DeriveSessionID(connectionID string) string {
	if connectionID == "" {
		return ""
	}
	sessionInit.Do(startSessionCleanup)
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if e := sessionStore[connectionID]; e != nil {
		e.lastUsed = time.Now()
		return e.id
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	sessionStore[connectionID] = &sessionEntry{id: id, lastUsed: time.Now()}
	return id
}

// ResetSessionID forces a new ID on the next DeriveSessionID call (used when
// upstream returns a session-invalid error).
func ResetSessionID(connectionID string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	delete(sessionStore, connectionID)
}

// ActiveSessionCount returns how many connections currently have a cached ID.
func ActiveSessionCount() int {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	return len(sessionStore)
}
