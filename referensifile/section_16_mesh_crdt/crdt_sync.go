// Package mesh — kernel/mesh/crdt_sync.go
//
// CRDT state replication via append-only event log + LWW resolve.
//
// Storage: <data>/mesh/events.jsonl (newline-delimited JSON Event records).
// Append-only on local mutation; merge-append on incoming peer events.
//
// Sync flow:
//   1. Local node tracks last-known HLC per peer (cache di settings DB
//      key MESH_PEER_SYNC_HLC_<peer_url_hash>).
//   2. On heartbeat tick: POST /v1/p2p/crdt/sync {since_hlc, events[]} ke
//      each peer (current logic: Phase E2-A2 push-only; pull via reverse
//      direction Phase E3 enhancement).
//   3. Peer respond echo + return their events since our last sync HLC.
//   4. Apply received events via Apply: HLC ordering + LWW conflict resolve.
//   5. Update last-known HLC per peer.
//
// Materialized view: latest-value-per-key built lazily via Snapshot().
// Caller (settings/warga apply hook) consume snapshot ke target table.

package mesh

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	kpath "github.com/flowork/kernel/kernel/path"
)

// ErrEventInvalid sentinel for malformed event input.
var ErrEventInvalid = errors.New("crdt: event invalid")

// EventLog = append-only file at <data>/mesh/events.jsonl.
//
// Phase E2-A2 MVP: pure file append. Phase E3 upgrade → SQLite kalau scale
// >10k events (linear scan jadi O(n) bottleneck).
type EventLog struct {
	mu   sync.Mutex
	path string
}

// OpenEventLog return *EventLog tied to <data>/mesh/events.jsonl. Auto-create
// parent dir + file (idempotent).
func OpenEventLog() (*EventLog, error) {
	dataDir, err := kpath.DataDir()
	if err != nil {
		return nil, fmt.Errorf("OpenEventLog: data dir: %w", err)
	}
	dir := filepath.Join(dataDir, "mesh")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("OpenEventLog: mkdir: %w", err)
	}
	return &EventLog{path: filepath.Join(dir, "events.jsonl")}, nil
}

// Path return absolute path (untuk debug + manual inspect).
func (l *EventLog) Path() string { return l.path }

// Append serialize event sebagai JSONL row + write atomically (file open
// O_APPEND, single Write call). Locked under mutex untuk concurrent local
// mutation.
func (l *EventLog) Append(ev Event) error {
	if ev.Key == "" {
		return fmt.Errorf("%w: empty key", ErrEventInvalid)
	}
	if ev.HLC.NodeID == "" {
		return fmt.Errorf("%w: empty HLC node_id", ErrEventInvalid)
	}

	row, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("EventLog.Append: marshal: %w", err)
	}
	row = append(row, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("EventLog.Append: open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(row); err != nil {
		return fmt.Errorf("EventLog.Append: write: %w", err)
	}
	return nil
}

// ReadSince stream events with HLC > since. since.PhysMs == 0 → return all.
//
// Linear scan — Phase E2-A2 MVP scale (<10k events). Caller responsible
// untuk paginate kalau response besar.
func (l *EventLog) ReadSince(since HLC) ([]Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // empty log
		}
		return nil, fmt.Errorf("EventLog.ReadSince: open: %w", err)
	}
	defer f.Close()

	var out []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip malformed line (ngga crash sync)
		}
		if since.PhysMs == 0 || ev.HLC.Compare(since) > 0 {
			out = append(out, ev)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("EventLog.ReadSince: scan: %w", err)
	}
	return out, nil
}

// Apply merge incoming events ke local log + return list yang actually
// di-apply (skip duplicate by HLC).
//
// LWW semantics: caller (Snapshot atau materialized view rebuilder) compute
// final state per-key dari full log dengan max-HLC-wins.
//
// Skew rejection: event dengan HLC.PhysMs > now+MaxClockSkewMs di-skip
// (anti malicious future-shift).
func (l *EventLog) Apply(events []Event) ([]Event, error) {
	if len(events) == 0 {
		return nil, nil
	}

	now := nowMs()
	applied := make([]Event, 0, len(events))

	// Build set of existing HLC strings untuk dedup (linear scan ngga
	// optimal; Phase E3 SQLite upgrade pake index).
	existing, err := l.ReadSince(HLC{}) // all
	if err != nil {
		return nil, fmt.Errorf("Apply: read existing: %w", err)
	}
	seen := make(map[string]bool, len(existing))
	for _, e := range existing {
		seen[e.HLC.String()] = true
	}

	for _, ev := range events {
		// Validate.
		if ev.Key == "" || ev.HLC.NodeID == "" {
			continue
		}
		// Skew reject.
		if ev.HLC.PhysMs > now+MaxClockSkewMs {
			continue
		}
		// Dedup.
		key := ev.HLC.String()
		if seen[key] {
			continue
		}
		seen[key] = true

		if err := l.Append(ev); err != nil {
			return applied, fmt.Errorf("Apply: append %s: %w", key, err)
		}
		applied = append(applied, ev)
	}
	return applied, nil
}

// Snapshot materialize current state per Key — LWW resolved (max HLC wins).
// Tombstone (Type="delete") removes key dari result. Iterates full log.
//
// Phase E2-A2 MVP: O(n) linear scan + map. Cache invalidation di Phase E3
// kalau scale demand.
func (l *EventLog) Snapshot() (map[string]string, error) {
	events, err := l.ReadSince(HLC{})
	if err != nil {
		return nil, err
	}

	// Latest HLC per key.
	type latest struct {
		hlc   HLC
		ev    Event
	}
	winner := make(map[string]latest, len(events)/2)
	for _, ev := range events {
		cur, exists := winner[ev.Key]
		if !exists || ev.HLC.Compare(cur.hlc) > 0 {
			winner[ev.Key] = latest{hlc: ev.HLC, ev: ev}
		}
	}

	out := make(map[string]string, len(winner))
	for k, l := range winner {
		if l.ev.Type == "delete" {
			continue // tombstone — key removed
		}
		out[k] = l.ev.Value
	}
	return out, nil
}

// SyncRequest payload for POST /v1/p2p/crdt/sync.
type SyncRequest struct {
	SinceHLC HLC     `json:"since_hlc"`
	Events   []Event `json:"events"`
}

// SyncResponse payload return.
type SyncResponse struct {
	Applied  int     `json:"applied"`
	Returned []Event `json:"returned"`
}

// HandleSync implement server-side handler logic — caller (HTTP handler)
// pass parsed SyncRequest, get SyncResponse. Decoupled from net/http.
//
// Side effects:
//  1. Apply incoming events to local log (LWW dedup).
//  2. Return events local has SINCE peer's last-known HLC (incremental).
func HandleSync(req SyncRequest) (SyncResponse, error) {
	log, err := OpenEventLog()
	if err != nil {
		return SyncResponse{}, fmt.Errorf("HandleSync: %w", err)
	}

	applied, err := log.Apply(req.Events)
	if err != nil {
		return SyncResponse{}, fmt.Errorf("HandleSync: apply: %w", err)
	}

	// Echo back local events since peer's HLC (incremental delta).
	returned, err := log.ReadSince(req.SinceHLC)
	if err != nil {
		return SyncResponse{}, fmt.Errorf("HandleSync: read since: %w", err)
	}

	return SyncResponse{
		Applied:  len(applied),
		Returned: returned,
	}, nil
}

// sharedEventLog singleton — lazy init untuk caller convenience (settings.Set
// hook calls SharedEventLog().Append directly).
var (
	sharedLog    *EventLog
	sharedLogMu  sync.Mutex
	sharedLogErr error
)

// SharedEventLog process-level singleton EventLog.
func SharedEventLog() *EventLog {
	sharedLogMu.Lock()
	defer sharedLogMu.Unlock()
	if sharedLog == nil && sharedLogErr == nil {
		sharedLog, sharedLogErr = OpenEventLog()
	}
	return sharedLog
}

// SharedEventLogErr last init error.
func SharedEventLogErr() error {
	sharedLogMu.Lock()
	defer sharedLogMu.Unlock()
	return sharedLogErr
}

// resetSharedLogForTest test helper — pair dengan FLOWORK_HOME=t.TempDir().
func resetSharedLogForTest() {
	sharedLogMu.Lock()
	defer sharedLogMu.Unlock()
	sharedLog = nil
	sharedLogErr = nil
}
