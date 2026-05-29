// Package lora — budget.go: daily byte budget enforcer untuk LoRa transmission.
//
// LoRa = SLOW + spectrum courtesy. Tanpa cap, malicious peer atau bug bisa
// burn semua air time monopoli channel. Default 100KB/peer/hari = ~12 menit
// total transmit time at 50kbps (worst case). Aman.
//
// Budget reset tiap calendar day (local timezone). Persistence opsional —
// MVP in-memory aja, restart kernel = reset (acceptable LoRa context).

package lora

import (
	"sync"
	"time"
)

// DailyBudget — per-peer daily byte cap.
type DailyBudget struct {
	mu        sync.Mutex
	limitDay  int                   // bytes per day
	usage     map[string]budgetState // peer pubkey hex → state
}

type budgetState struct {
	day   string // YYYY-MM-DD local
	bytes int
}

// DefaultDailyLimitBytes — 100 KB per peer per day.
const DefaultDailyLimitBytes = 100 * 1024

// NewDailyBudget construct dengan limit. limit <= 0 → use default 100KB.
func NewDailyBudget(limitBytes int) *DailyBudget {
	if limitBytes <= 0 {
		limitBytes = DefaultDailyLimitBytes
	}
	return &DailyBudget{
		limitDay: limitBytes,
		usage:    map[string]budgetState{},
	}
}

// CanSend return true kalau peer masih punya budget untuk size bytes hari ini.
func (b *DailyBudget) CanSend(peer string, size int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	state := b.usage[peer]
	if state.day != today {
		state = budgetState{day: today, bytes: 0}
	}
	return state.bytes+size <= b.limitDay
}

// Record account size bytes ke budget peer (call after successful send).
func (b *DailyBudget) Record(peer string, size int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	state := b.usage[peer]
	if state.day != today {
		state = budgetState{day: today, bytes: 0}
	}
	state.bytes += size
	b.usage[peer] = state
}

// Used return total bytes used today untuk peer.
func (b *DailyBudget) Used(peer string) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	state := b.usage[peer]
	if state.day != today {
		return 0
	}
	return state.bytes
}

// Remaining return bytes left untuk peer hari ini.
func (b *DailyBudget) Remaining(peer string) int {
	used := b.Used(peer)
	if used >= b.limitDay {
		return 0
	}
	return b.limitDay - used
}

// Limit return configured daily limit.
func (b *DailyBudget) Limit() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.limitDay
}

// Reset clear all per-peer state (debug / test).
func (b *DailyBudget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.usage = map[string]budgetState{}
}
