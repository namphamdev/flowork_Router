// Package lora — priority.go: priority queue 6-tier untuk LoRa delta send order.
//
// LoRa bandwidth ~250bps-50kbps = SLOW. Kalau kirim drawer telemetry duluan,
// constitution update sakral nyangkut di belakang antrian berhari-hari.
//
// Priority order (lowest enum = highest priority, P0 drain first):
//
//	P0 EMERGENCY     — Master signed alert (M18 emergency_alert), bypass queue
//	P1 CONSTITUTION  — sakral doctrine update (amplitude 999999)
//	P2 SKILL         — curated knowledge skill content
//	P3 DRAWER        — raw memory drawer chunks
//	P4 CACHED_REASON — deduplicated LLM reasoning cache
//	P5 TELEMETRY     — codemap, metrics, log (lowest, can wait days)
//
// Threadsafe via mutex. Pop blocks until item available atau ctx Done.

package lora

import (
	"context"
	"sync"
)

// Priority enum.
type Priority int

const (
	P0Emergency Priority = iota
	P1Constitution
	P2Skill
	P3Drawer
	P4CachedReason
	P5Telemetry

	NumPriorities = 6
)

// String human-readable nama.
func (p Priority) String() string {
	switch p {
	case P0Emergency:
		return "EMERGENCY"
	case P1Constitution:
		return "CONSTITUTION"
	case P2Skill:
		return "SKILL"
	case P3Drawer:
		return "DRAWER"
	case P4CachedReason:
		return "CACHED_REASON"
	case P5Telemetry:
		return "TELEMETRY"
	default:
		return "UNKNOWN"
	}
}

// QueuedItem — entry di priority queue.
type QueuedItem struct {
	Priority Priority
	Payload  []byte // serialized DeltaChunk atau Frame payload
	Meta     map[string]string // optional debug metadata (packet_id, hlc, etc)
}

// PriorityQueue — 6-tier FIFO queue. Higher priority drain dulu.
type PriorityQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	queues [NumPriorities][]QueuedItem
	closed bool
}

// NewPriorityQueue construct.
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{}
	pq.cond = sync.NewCond(&pq.mu)
	return pq
}

// Push enqueue item ke tier sesuai Priority. Wake up satu Pop waiter.
func (pq *PriorityQueue) Push(item QueuedItem) {
	if int(item.Priority) >= NumPriorities || int(item.Priority) < 0 {
		item.Priority = P5Telemetry // default lowest
	}
	pq.mu.Lock()
	pq.queues[item.Priority] = append(pq.queues[item.Priority], item)
	pq.cond.Signal()
	pq.mu.Unlock()
}

// Pop block until item available atau ctx done. Drain higher priority first.
//
// Returns item + true kalau OK, zero-value + false kalau ctx done atau closed.
func (pq *PriorityQueue) Pop(ctx context.Context) (QueuedItem, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Wake on context cancel via goroutine
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			pq.mu.Lock()
			pq.cond.Broadcast()
			pq.mu.Unlock()
		case <-stop:
		}
	}()

	for {
		if pq.closed {
			return QueuedItem{}, false
		}
		if ctx.Err() != nil {
			return QueuedItem{}, false
		}
		// Drain highest priority first
		for p := P0Emergency; p < NumPriorities; p++ {
			if len(pq.queues[p]) > 0 {
				item := pq.queues[p][0]
				pq.queues[p] = pq.queues[p][1:]
				return item, true
			}
		}
		pq.cond.Wait()
	}
}

// TryPop non-blocking variant. Returns false kalau queue kosong.
func (pq *PriorityQueue) TryPop() (QueuedItem, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for p := P0Emergency; p < NumPriorities; p++ {
		if len(pq.queues[p]) > 0 {
			item := pq.queues[p][0]
			pq.queues[p] = pq.queues[p][1:]
			return item, true
		}
	}
	return QueuedItem{}, false
}

// Len total item count across all priorities.
func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	n := 0
	for _, q := range pq.queues {
		n += len(q)
	}
	return n
}

// LenByPriority return per-tier count (untuk debug + GUI dashboard).
func (pq *PriorityQueue) LenByPriority() [NumPriorities]int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	var counts [NumPriorities]int
	for i, q := range pq.queues {
		counts[i] = len(q)
	}
	return counts
}

// Close mark queue closed, wake all waiters dengan false return.
func (pq *PriorityQueue) Close() {
	pq.mu.Lock()
	pq.closed = true
	pq.cond.Broadcast()
	pq.mu.Unlock()
}
