// Package mesh — kernel/mesh/crdt_event.go
//
// CRDT event types + Hybrid Logical Clock (HLC) per opus-3 spec.
//
// HLC pattern: physical ms timestamp + 16-bit logical counter + node ID
// tiebreaker. Properties:
//   - Monotonic per node (logical counter handles same-ms collision)
//   - Comparable globally (deterministic ordering across nodes)
//   - Skew-tolerant (defensive cap ±10 min wall-clock divergence)
//
// Reference: CockroachDB HLC paper "Logical Physical Clocks", standard
// pattern for distributed state replication.
//
// Event log: append-only JSONL at <data>/mesh/events.jsonl. Sync filters
// by HLC for incremental replication. Scale upgrade path → SQLite kalau
// >10k events.

package mesh

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// HLC = Hybrid Logical Clock entry.
//
// Wire format: JSON tuple `[phys_ms, logical, node_id]` — kompak, ordered,
// no map key serialization variability (signature-stable).
type HLC struct {
	PhysMs  int64  // unix ms (48-bit usable, ~year 10889 cap)
	Logical uint16 // 16-bit collision counter
	NodeID  string // lex tiebreaker
}

// MaxClockSkewMs defensive cap — refuse event > 10 min in future
// (anti malicious node spam future timestamp).
const MaxClockSkewMs int64 = 10 * 60 * 1000

// MarshalJSON encode tuple `[phys_ms, logical, node_id]`.
func (h HLC) MarshalJSON() ([]byte, error) {
	return json.Marshal([3]any{h.PhysMs, h.Logical, h.NodeID})
}

// UnmarshalJSON decode tuple `[phys_ms, logical, node_id]`.
func (h *HLC) UnmarshalJSON(data []byte) error {
	var arr [3]any
	if err := json.Unmarshal(data, &arr); err != nil {
		return fmt.Errorf("HLC unmarshal: %w", err)
	}

	// JSON numbers default float64.
	phys, ok := arr[0].(float64)
	if !ok {
		return errors.New("HLC unmarshal: phys_ms not number")
	}
	logical, ok := arr[1].(float64)
	if !ok {
		return errors.New("HLC unmarshal: logical not number")
	}
	if logical < 0 || logical > 65535 {
		return fmt.Errorf("HLC unmarshal: logical %v out of uint16 range", logical)
	}
	nodeID, ok := arr[2].(string)
	if !ok {
		return errors.New("HLC unmarshal: node_id not string")
	}

	h.PhysMs = int64(phys)
	h.Logical = uint16(logical)
	h.NodeID = nodeID
	return nil
}

// Compare return -1/0/+1 (h vs other) per total order: PhysMs → Logical → NodeID lex.
func (h HLC) Compare(other HLC) int {
	if h.PhysMs < other.PhysMs {
		return -1
	}
	if h.PhysMs > other.PhysMs {
		return 1
	}
	if h.Logical < other.Logical {
		return -1
	}
	if h.Logical > other.Logical {
		return 1
	}
	return strings.Compare(h.NodeID, other.NodeID)
}

// String human-readable HLC representation.
func (h HLC) String() string {
	return fmt.Sprintf("%d.%d.%s", h.PhysMs, h.Logical, h.NodeID)
}

// Clock = thread-safe HLC instance per kernel.
type Clock struct {
	mu     sync.Mutex
	state  HLC
	nodeID string
}

// NewClock create Clock untuk nodeID. PhysMs initialized ke now, Logical 0.
func NewClock(nodeID string) *Clock {
	return &Clock{
		state:  HLC{PhysMs: nowMs(), Logical: 0, NodeID: nodeID},
		nodeID: nodeID,
	}
}

// Tick advance clock untuk local event (no peer event input).
//
// Rules:
//   - now > state.PhysMs → reset Logical=0, set PhysMs=now
//   - now <= state.PhysMs → Logical++ (same-ms collision OR backward skew)
//
// Logical overflow check: kalau hit max uint16 65535, tunggu next ms (caller
// responsibility — atau skip tick, return same state). Defensive: log + continue.
func (c *Clock) Tick() HLC {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := nowMs()
	if now > c.state.PhysMs {
		c.state.PhysMs = now
		c.state.Logical = 0
	} else {
		// same-ms or skew → logical++
		if c.state.Logical < 65535 {
			c.state.Logical++
		}
		// else saturate (rare — would need 65536 events in same ms)
	}
	return c.state
}

// Update merge peer HLC into local clock (call saat receive event dari peer).
//
// Rules per HLC paper:
//   - Take max(local.PhysMs, peer.PhysMs, now)
//   - Logical: complex — kalau peer dominates, take peer.Logical+1; etc.
//
// Reject: kalau peer.PhysMs > now + MaxClockSkewMs → log + reject (anti
// malicious future-shift). Caller responsibility check return error.
func (c *Clock) Update(peer HLC) (HLC, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := nowMs()
	if peer.PhysMs > now+MaxClockSkewMs {
		return c.state, fmt.Errorf("Clock.Update: peer phys %d > now+%dms (skew rejected)",
			peer.PhysMs, MaxClockSkewMs)
	}

	// Pick max of three (now, local, peer).
	maxPhys := now
	if c.state.PhysMs > maxPhys {
		maxPhys = c.state.PhysMs
	}
	if peer.PhysMs > maxPhys {
		maxPhys = peer.PhysMs
	}

	switch {
	case maxPhys == c.state.PhysMs && maxPhys == peer.PhysMs:
		// All three equal → max(local.Logical, peer.Logical) + 1
		if c.state.Logical < peer.Logical {
			c.state.Logical = peer.Logical + 1
		} else {
			c.state.Logical++
		}
	case maxPhys == c.state.PhysMs:
		// Local dominates → logical++
		c.state.Logical++
	case maxPhys == peer.PhysMs:
		// Peer dominates → adopt peer.PhysMs + peer.Logical+1
		c.state.PhysMs = peer.PhysMs
		c.state.Logical = peer.Logical + 1
	default:
		// now dominates → reset
		c.state.PhysMs = maxPhys
		c.state.Logical = 0
	}

	return c.state, nil
}

// Now snapshot current state without tick.
func (c *Clock) Now() HLC {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Event = single CRDT mutation on a key.
//
// Operation: LWW (Last-Write-Wins) — caller (CRDT sync engine) compare HLC
// across events for same Key; latest-HLC wins.
//
// Tombstone: empty Value + Type="delete" represent key removal. Caller
// preserve tombstone history for at least sync window (anti rejuvenation).
type Event struct {
	HLC   HLC    `json:"hlc"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"` // "set" (default) | "delete"
}

// nowMs current unix ms — testable seam (test override via clock injection
// kalau perlu deterministic timing).
var nowMs = func() int64 {
	return time.Now().UnixMilli()
}
