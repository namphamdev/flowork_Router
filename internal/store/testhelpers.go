// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/store package — audit pass surface review.

package store

import "sync"

// resetDBSingletonForTest clears the package-level singleton so tests can swap
// FLOW_ROUTER_DATA env and obtain a fresh DB. Not exported; only callers in
// the same package (tests) reach it.
func resetDBSingletonForTest() {
	if db != nil {
		_ = db.Close()
	}
	db = nil
	dbErr = nil
	dbOnce = sync.Once{}
}
