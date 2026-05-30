// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file. Standard test patterns, no production race/leak risk.

package router

import (
	"sync"
	"testing"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// TestPickComboModel_RoundRobinRace exercises pickComboModel concurrently to
// prove the global roundRobinCursor map is accessed without synchronization.
// Run with `go test -race ./internal/router/ -run TestPickComboModel_RoundRobinRace`.
func TestPickComboModel_RoundRobinRace(t *testing.T) {
	combo := &store.Combo{
		ID:       "combo-race",
		Strategy: store.ComboStrategyRoundRobin,
		Models:   []string{"a", "b", "c", "d"},
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = pickComboModel(combo)
			}
		}()
	}
	wg.Wait()
}
