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
