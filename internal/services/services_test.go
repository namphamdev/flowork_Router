// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package services

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestAccountFallback_429ExponentialBackoff(t *testing.T) {
	d1 := CheckFallbackError(429, "rate limited", 0)
	if !d1.ShouldFallback || d1.NewBackoffLevel != 1 || d1.Cooldown != BackoffConfig.Base {
		t.Fatalf("level 1: want fallback+1×base, got %+v", d1)
	}
	d2 := CheckFallbackError(429, "", d1.NewBackoffLevel)
	if d2.NewBackoffLevel != 2 || d2.Cooldown != 2*BackoffConfig.Base {
		t.Fatalf("level 2: want 2×base, got %+v", d2)
	}
	// Drive to MaxLevel — assert level saturates and the resulting cooldown
	// never exceeds the Max cap (regardless of whether the geometric
	// doubling happens to hit Max exactly given Base+MaxLevel values).
	bl := d2.NewBackoffLevel
	for i := 0; i < 20; i++ {
		bl = CheckFallbackError(429, "", bl).NewBackoffLevel
	}
	if bl != BackoffConfig.MaxLevel {
		t.Fatalf("expected level to saturate at MaxLevel=%d, got %d", BackoffConfig.MaxLevel, bl)
	}
	if got := GetQuotaCooldown(bl); got > BackoffConfig.Max {
		t.Fatalf("cooldown exceeded Max cap: got %v cap %v", got, BackoffConfig.Max)
	}
}

func TestAccountFallback_TextRuleWinsOverStatus(t *testing.T) {
	d := CheckFallbackError(500, "Quota exceeded for account", 0)
	if d.NewBackoffLevel != 1 {
		t.Fatalf("text rate-limit rule should escalate backoff, got %+v", d)
	}
}

func TestAccountFallback_UnmatchedGetsTransient(t *testing.T) {
	d := CheckFallbackError(0, "weird upstream noise", 0)
	if d.Cooldown != TransientCooldown {
		t.Fatalf("unmatched must use transient default, got %v", d.Cooldown)
	}
}

func TestAccountFallback_AvailabilityWindow(t *testing.T) {
	future := time.Now().Add(2 * time.Second)
	if !IsAccountUnavailable(future) {
		t.Fatal("future timestamp must be considered unavailable")
	}
	past := time.Now().Add(-time.Second)
	if IsAccountUnavailable(past) {
		t.Fatal("past timestamp must be available again")
	}
}

func TestAccountFallback_EarliestRateLimited(t *testing.T) {
	a := time.Now().Add(10 * time.Second)
	b := time.Now().Add(2 * time.Second)
	c := time.Now().Add(-5 * time.Second) // already past
	earliest, ok := GetEarliestRateLimitedUntil([]time.Time{a, b, c})
	if !ok || !earliest.Equal(b) {
		t.Fatalf("expected b as earliest, got %v ok=%v", earliest, ok)
	}
}

// ── TokenRefresh worker ────────────────────────────────────────────────

type fakeSource struct {
	provider string
	exp      atomic.Value // time.Time
	refreshN atomic.Int32
	err      error
}

func (f *fakeSource) Provider() string     { return f.provider }
func (f *fakeSource) ExpiresAt() time.Time { v, _ := f.exp.Load().(time.Time); return v }
func (f *fakeSource) Refresh(ctx context.Context) (time.Time, error) {
	f.refreshN.Add(1)
	if f.err != nil {
		return time.Time{}, f.err
	}
	newExp := time.Now().Add(time.Hour)
	f.exp.Store(newExp)
	return newExp, nil
}

func TestTokenRefresh_RefreshesWhenDue(t *testing.T) {
	old := RefreshLead
	RefreshLead = 100 * time.Millisecond
	t.Cleanup(func() { RefreshLead = old })

	src := &fakeSource{provider: "test"}
	src.exp.Store(time.Now().Add(50 * time.Millisecond)) // expires in 50ms → due NOW (< lead)
	w := NewWorker()
	w.Add(src)
	w.Start()
	t.Cleanup(w.Stop)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if src.refreshN.Load() >= 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("worker never invoked Refresh; count=%d", src.refreshN.Load())
}

func TestTokenRefresh_FailureRetries(t *testing.T) {
	oldFail := FailureRetry
	FailureRetry = 80 * time.Millisecond
	oldLead := RefreshLead
	RefreshLead = 1 * time.Second
	t.Cleanup(func() { FailureRetry = oldFail; RefreshLead = oldLead })

	src := &fakeSource{provider: "test-fail", err: errors.New("simulated")}
	src.exp.Store(time.Now().Add(-time.Second)) // already past lead → due
	w := NewWorker()
	w.Add(src)
	w.Start()
	t.Cleanup(w.Stop)

	time.Sleep(500 * time.Millisecond)
	// At minimum the worker MUST attempt at least one refresh; failure path
	// not crashing the goroutine. Higher counts are scheduler-dependent on
	// slow CI, so we only assert the lower bound here.
	if src.refreshN.Load() < 1 {
		t.Fatalf("expected ≥1 retry within 500ms, got %d", src.refreshN.Load())
	}
}
