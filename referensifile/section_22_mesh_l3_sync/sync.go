// Package mesh provides peer-to-peer mesh networking for FLOWORK agents.
// Enables offline-first communication between agents on the same LAN or
// via extended-range transports (LoRa, Wi-Fi Direct).
package mesh

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HashChainEntry represents a single message in the append-only log
// with a hash chain for deduplication and integrity verification.
type HashChainEntry struct {
	// Hash is SHA-256 of (PrevHash + Timestamp + From + Channel + Message)
	Hash string `json:"hash"`
	// PrevHash links this entry to the previous one (Merkle chain)
	PrevHash string `json:"prev_hash"`

	// Message content (same format as shared_chat.jsonl)
	Timestamp string `json:"ts"`
	From      string `json:"from"`
	Channel   string `json:"channel"`
	Message   string `json:"message"`

	// PeerID identifies which peer originated this message
	PeerID string `json:"peer_id,omitempty"`
}

// computeHash computes the SHA-256 hash for an entry based on its content
// and the previous hash in the chain.
// Uses null byte (\x00) as separator to avoid collision when message content
// contains the delimiter character.
func computeHash(prevHash, ts, from, channel, message string) string {
	data := prevHash + "\x00" + ts + "\x00" + from + "\x00" + channel + "\x00" + message
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// SyncLog manages the append-only hash-chain log for peer-to-peer
// message synchronization. It provides deduplication via hash lookup
// and integrity verification via the Merkle chain.
//
// EXTBUG-013: in-memory `entries` + `hashSet` previously grew without
// bound — months of sync produced hundreds of MB RAM. We now cap both
// via MaxInMemoryEntries; overflow drops the oldest 20% from memory
// (disk log remains intact so peers re-syncing deep history can still
// tail the file).
type SyncLog struct {
	mu       sync.RWMutex
	entries  []HashChainEntry
	hashSet  map[string]bool // fast lookup for dedup
	lastHash string          // tip of the hash chain
	filePath string          // backing JSONL file
}

// MaxInMemoryEntries bounds resident set for SyncLog. 5000 entries ≈ a
// month of moderate team chatter; tune via setter in future.
const MaxInMemoryEntries = 5000

// NewSyncLog creates a new sync log backed by the given file path.
// If the file exists, it loads existing entries.
func NewSyncLog(filePath string) (*SyncLog, error) {
	sl := &SyncLog{
		hashSet:  make(map[string]bool),
		filePath: filePath,
	}
	if err := sl.loadFromDisk(); err != nil {
		return nil, fmt.Errorf("load sync log: %w", err)
	}
	return sl, nil
}

// loadFromDisk reads the JSONL file and rebuilds the in-memory state.
func (sl *SyncLog) loadFromDisk() error {
	f, err := os.Open(sl.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // empty log is valid
		}
		return fmt.Errorf("mesh sync: open log %s: %w", sl.filePath, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		var entry HashChainEntry
		if err := json.Unmarshal(sc.Bytes(), &entry); err != nil {
			continue // skip malformed lines
		}
		sl.entries = append(sl.entries, entry)
		sl.hashSet[entry.Hash] = true
		sl.lastHash = entry.Hash
	}
	return sc.Err()
}

// Append adds a new message to the log. Returns the computed hash.
// If the message hash already exists (duplicate), it's a no-op.
func (sl *SyncLog) Append(from, channel, message, peerID string) (string, error) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	ts := time.Now().UTC().Format(time.RFC3339Nano)
	hash := computeHash(sl.lastHash, ts, from, channel, message)

	// Dedup check
	if sl.hashSet[hash] {
		return hash, nil // already have this message
	}

	entry := HashChainEntry{
		Hash:      hash,
		PrevHash:  sl.lastHash,
		Timestamp: ts,
		From:      from,
		Channel:   channel,
		Message:   message,
		PeerID:    peerID,
	}

	// Append to file
	if err := sl.appendToDisk(entry); err != nil {
		return "", fmt.Errorf("write entry: %w", err)
	}

	sl.entries = append(sl.entries, entry)
	sl.hashSet[hash] = true
	sl.lastHash = hash
	sl.trimInMemory()
	return hash, nil
}

// trimInMemory enforces MaxInMemoryEntries by evicting the oldest 20% once
// we cross the cap. Must be called under sl.mu write lock. EXTBUG-013.
func (sl *SyncLog) trimInMemory() {
	if len(sl.entries) <= MaxInMemoryEntries {
		return
	}
	evictN := MaxInMemoryEntries / 5
	for i := 0; i < evictN; i++ {
		delete(sl.hashSet, sl.entries[i].Hash)
	}
	// Shift remaining entries to new backing slice so old underlying array
	// can be GC'd instead of keeping the full capacity pinned.
	remaining := make([]HashChainEntry, len(sl.entries)-evictN)
	copy(remaining, sl.entries[evictN:])
	sl.entries = remaining
}

// appendToDisk writes a single entry to the backing file.
func (sl *SyncLog) appendToDisk(entry HashChainEntry) error {
	_ = os.MkdirAll(filepath.Dir(sl.filePath), 0o755)
	f, err := os.OpenFile(sl.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("mesh sync: open log for append: %w", err)
	}
	defer f.Close()
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("mesh sync: marshal entry: %w", err)
	}
	_, err = fmt.Fprintf(f, "%s\n", b)
	if err != nil {
		return fmt.Errorf("mesh sync: write entry: %w", err)
	}
	return nil
}

// HasHash returns true if the log already contains an entry with the given hash.
func (sl *SyncLog) HasHash(hash string) bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return sl.hashSet[hash]
}

// EntriesSince returns all entries after the given hash.
// If hash is empty, returns all entries.
// Used during peer sync to send only missing entries.
func (sl *SyncLog) EntriesSince(afterHash string) []HashChainEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if afterHash == "" {
		copied := make([]HashChainEntry, len(sl.entries))
		copy(copied, sl.entries)
		return copied
	}

	found := false
	var result []HashChainEntry
	for _, e := range sl.entries {
		if found {
			result = append(result, e)
		}
		// rc135 timing-attack audit: NOT timing-sensitive. Entry.Hash is a
		// public SHA256 digest; collision requires preimage attack (feasibility
		// << O(2^256)). Same-side timing leak tidak bantu attacker — digest
		// sudah diterbitkan sebelum compare. Plain == adalah benar di sini.
		if e.Hash == afterHash {
			found = true
		}
	}
	return result
}

// MergeEntries merges entries received from a peer into the local log.
// Only entries with unknown hashes are appended (dedup).
// Returns the number of new entries merged.
//
// Gemini audit fix (Bug 8.2 — False Memory / Merkle Poisoning):
// previously entries were accepted blindly on hash novelty alone — a
// malicious peer could send {Hash: "precomputed", Message: "poisoned"}
// and corrupt the chain. Now each entry is verified:
//
//  1. Hash must match computeHash(PrevHash, Timestamp, From, Channel, Message).
//     If not, entry is rejected (skipped, error appended).
//  2. PrevHash must chain to either our current tip or an entry already
//     accepted earlier in the same batch — so isolated/forged entries
//     with random PrevHash are refused.
//
// Rejected entries do NOT abort the merge; valid entries that follow are
// still appended. The error return describes the first rejection for
// caller logging.
func (sl *SyncLog) MergeEntries(entries []HashChainEntry) (int, error) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	merged := 0
	var firstRejection error
	for _, entry := range entries {
		if sl.hashSet[entry.Hash] {
			continue // already have this one
		}
		// 1. Integrity: hash must cover the payload.
		// rc135 timing-attack audit: NOT timing-sensitive. Hash integrity
		// compare antar public digest (SHA256). Attacker tidak dapat info
		// baru dari timing — kedua hash publicly knowable dari payload.
		expectedHash := computeHash(entry.PrevHash, entry.Timestamp, entry.From, entry.Channel, entry.Message)
		if entry.Hash != expectedHash {
			if firstRejection == nil {
				firstRejection = fmt.Errorf("rejected forged entry (hash mismatch): got %s want %s", short8(entry.Hash), short8(expectedHash))
			}
			continue
		}
		// 2. Chain continuity: PrevHash must equal current tip, OR be a
		// known ancestor (genesis is "" on an empty chain).
		if !sl.isChainContinuous(entry.PrevHash) {
			if firstRejection == nil {
				firstRejection = fmt.Errorf("rejected orphan entry %s: PrevHash %s not in local chain", short8(entry.Hash), short8(entry.PrevHash))
			}
			continue
		}
		if err := sl.appendToDisk(entry); err != nil {
			return merged, fmt.Errorf("merge entry %s: %w", short8(entry.Hash), err)
		}
		sl.entries = append(sl.entries, entry)
		sl.hashSet[entry.Hash] = true
		// Gemini audit #4 (DAG fork tip overwrite): only advance lastHash when
		// the merged entry genuinely extends our current tip. When it merely
		// chains from an older ancestor (fork from a peer's divergent branch),
		// keep our local tip so subsequent Append() stays on our chain — the
		// forked entry is still stored for history + dedup, but doesn't hijack
		// tip progression. Multi-tip DAG tracking is a v0.5 item.
		//
		// Audit #8 follow-up (genesis collision): when both sl.lastHash AND
		// entry.PrevHash are "" (fresh peer sync, entry IS genesis), the
		// equality holds — lastHash advances correctly. If entry.PrevHash is
		// "" but sl.lastHash is not (we already have a chain), don't advance
		// (peer replayed their genesis entry back at us).
		// rc135 timing-attack audit: NOT timing-sensitive. Chain tip advance
		// — PrevHash dan lastHash keduanya public digest, plain == benar.
		if entry.PrevHash == sl.lastHash {
			sl.lastHash = entry.Hash
		}
		merged++
	}
	return merged, firstRejection
}

// isChainContinuous reports whether prevHash is a valid predecessor for a
// new entry: either the current tip, a previously-seen hash, or empty
// string when the local chain is still empty (genesis).
func (sl *SyncLog) isChainContinuous(prevHash string) bool {
	if prevHash == "" {
		return sl.lastHash == "" && len(sl.entries) == 0
	}
	// rc135 timing-attack audit: NOT timing-sensitive. Chain hash lookup,
	// tidak ada secret di kedua operand (public digest).
	if prevHash == sl.lastHash {
		return true
	}
	return sl.hashSet[prevHash]
}

func short8(s string) string {
	if len(s) < 8 {
		return s
	}
	return s[:8]
}

// Tip returns the hash of the most recent entry in the chain.
func (sl *SyncLog) Tip() string {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return sl.lastHash
}

// Len returns the total number of entries in the log.
func (sl *SyncLog) Len() int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return len(sl.entries)
}
