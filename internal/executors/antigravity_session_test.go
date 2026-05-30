// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package executors

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGenerateAntigravitySessionID_FormatLooksLikeUUIDPlusMillis(t *testing.T) {
	id := GenerateAntigravitySessionID()
	// Expect 36-char UUID + at least 13-digit millis suffix.
	if len(id) < 36+13 {
		t.Fatalf("session id too short (%d): %q", len(id), id)
	}
	uuidPart := id[:36]
	if uuidPart[8] != '-' || uuidPart[13] != '-' || uuidPart[18] != '-' || uuidPart[23] != '-' {
		t.Fatalf("UUID prefix dashes missing: %q", uuidPart)
	}
	// Version nibble must be '4' (RFC 4122).
	if uuidPart[14] != '4' {
		t.Fatalf("UUID v4 version nibble wrong: %q", uuidPart[14])
	}
	// Variant nibble must be 8/9/a/b.
	switch uuidPart[19] {
	case '8', '9', 'a', 'b':
	default:
		t.Fatalf("UUID variant nibble wrong: %q", uuidPart[19])
	}
	// Tail must be all digits.
	for _, c := range id[36:] {
		if c < '0' || c > '9' {
			t.Fatalf("non-digit char in millis tail: %q", id[36:])
		}
	}
}

func TestGenerateAntigravitySessionID_UniqueAcrossCalls(t *testing.T) {
	a := GenerateAntigravitySessionID()
	b := GenerateAntigravitySessionID()
	if a == b {
		t.Fatalf("two consecutive calls returned same id: %q", a)
	}
}

func TestDeriveAntigravitySessionID_StableForSameConnection(t *testing.T) {
	ClearAntigravitySessionStore()
	first := DeriveAntigravitySessionID("conn-abc")
	second := DeriveAntigravitySessionID("conn-abc")
	if first != second {
		t.Fatalf("stable cache lost on repeat: %q vs %q", first, second)
	}
}

func TestDeriveAntigravitySessionID_DistinctAcrossConnections(t *testing.T) {
	ClearAntigravitySessionStore()
	a := DeriveAntigravitySessionID("conn-A")
	b := DeriveAntigravitySessionID("conn-B")
	if a == b {
		t.Fatalf("different connections should NOT share a session id")
	}
}

func TestDeriveAntigravitySessionID_EmptyConnectionIsOneShot(t *testing.T) {
	ClearAntigravitySessionStore()
	a := DeriveAntigravitySessionID("")
	b := DeriveAntigravitySessionID("")
	if a == b {
		t.Fatalf("empty connectionId must not be cached: got same id twice (%q)", a)
	}
	if AntigravitySessionStoreSize() != 0 {
		t.Fatalf("empty connectionId must not populate store, size=%d", AntigravitySessionStoreSize())
	}
}

func TestDeriveAntigravitySessionID_ThreadSafe(t *testing.T) {
	ClearAntigravitySessionStore()
	const goroutines = 32
	results := make([]string, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = DeriveAntigravitySessionID("shared-conn")
		}(i)
	}
	wg.Wait()
	// All goroutines must observe the same id (whichever one won the race).
	first := results[0]
	for i, got := range results {
		if got != first {
			t.Fatalf("goroutine %d saw a different id: %q vs %q", i, got, first)
		}
	}
}

func TestClearAntigravitySessionStore_Resets(t *testing.T) {
	DeriveAntigravitySessionID("temp")
	if AntigravitySessionStoreSize() == 0 {
		t.Fatal("store should have at least 1 entry after derive")
	}
	ClearAntigravitySessionStore()
	if got := AntigravitySessionStoreSize(); got != 0 {
		t.Fatalf("clear should empty the store, size=%d", got)
	}
}

func TestAntigravitySessionStore_HasUnderlyingTimestamps(t *testing.T) {
	// Touch the same key after a small gap — lastUsed should advance.
	ClearAntigravitySessionStore()
	_ = DeriveAntigravitySessionID("touch")
	sessionMu.Lock()
	first := sessionStore["touch"].lastUsed
	sessionMu.Unlock()
	time.Sleep(10 * time.Millisecond)
	_ = DeriveAntigravitySessionID("touch")
	sessionMu.Lock()
	second := sessionStore["touch"].lastUsed
	sessionMu.Unlock()
	if !second.After(first) {
		t.Fatalf("lastUsed should advance: first=%v second=%v", first, second)
	}
}

func TestDeriveAntigravitySessionID_MillisTailContainsTimestamp(t *testing.T) {
	// Sanity: the timestamp portion should be close to "now".
	before := time.Now().UnixMilli()
	id := GenerateAntigravitySessionID()
	after := time.Now().UnixMilli()
	tail := id[36:]
	var got int64
	for _, c := range tail {
		got = got*10 + int64(c-'0')
	}
	if got < before-10 || got > after+10 {
		t.Fatalf("millis tail %d outside expected window [%d, %d]", got, before, after)
	}
	if !strings.HasSuffix(id, tail) {
		t.Fatal("internal split assumption broken")
	}
}
