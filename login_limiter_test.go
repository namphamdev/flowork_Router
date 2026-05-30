// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package main

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginLimiter_LocksAfterThreshold(t *testing.T) {
	// Use a synthetic IP key isolated from other tests.
	ip := "test-ip-1"
	loginRecordSuccess(ip) // reset state

	for i := 0; i < loginMaxFailsBeforeLock-1; i++ {
		locked, _ := loginRecordFail(ip)
		if locked {
			t.Fatalf("locked too early at attempt %d", i+1)
		}
	}
	locked, retry := loginRecordFail(ip)
	if !locked {
		t.Fatal("expected lock at threshold attempt")
	}
	if retry < 1 {
		t.Fatalf("retryAfter must be >=1, got %d", retry)
	}

	// CheckLock should now report locked.
	isLocked, _ := loginCheckLock(ip)
	if !isLocked {
		t.Fatal("expected loginCheckLock to report locked")
	}
}

func TestLoginLimiter_SuccessClearsState(t *testing.T) {
	ip := "test-ip-2"
	loginRecordSuccess(ip) // reset

	loginRecordFail(ip)
	loginRecordFail(ip)
	loginRecordSuccess(ip) // clear

	if locked, _ := loginCheckLock(ip); locked {
		t.Fatal("expected NOT locked after successful login clears state")
	}
	// And next fail starts fresh — should not lock immediately.
	if locked, _ := loginRecordFail(ip); locked {
		t.Fatal("expected fresh counter after success — should not lock on first fail")
	}
}

func TestLoginLimiter_ProgressiveBackoff(t *testing.T) {
	ip := "test-ip-3"
	loginRecordSuccess(ip)

	// Trigger first lock
	for i := 0; i < loginMaxFailsBeforeLock; i++ {
		loginRecordFail(ip)
	}
	loginLockMu.Lock()
	level1 := loginLocks[ip].lockLevel
	loginLockMu.Unlock()

	// Force expiry, then trigger again
	loginLockMu.Lock()
	loginLocks[ip].lockUntil = time.Now().Add(-time.Second)
	loginLockMu.Unlock()

	for i := 0; i < loginMaxFailsBeforeLock; i++ {
		loginRecordFail(ip)
	}
	loginLockMu.Lock()
	level2 := loginLocks[ip].lockLevel
	loginLockMu.Unlock()

	if level2 <= level1 {
		t.Fatalf("lockLevel must escalate: level1=%d level2=%d", level1, level2)
	}
}

func TestClientIPForLock_StripsPort(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.RemoteAddr = "192.168.1.50:54321"
	if got := clientIPForLock(r); got != "192.168.1.50" {
		t.Fatalf("expected 192.168.1.50, got %q", got)
	}
}
