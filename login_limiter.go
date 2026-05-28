// Dashboard Login Rate Limiter (in-memory).

package main

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// clientIPForLock returns the host portion of r.RemoteAddr, falling back to the
// raw value when SplitHostPort fails (e.g. unix sockets or test fakes).
func clientIPForLock(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

// strconvItoa is a tiny local alias so handlers_auth.go does not need to import
// strconv just for the Retry-After header value.
func strconvItoa(n int) string { return strconv.Itoa(n) }

const (
	loginMaxFailsBeforeLock = 5
	loginFailWindow         = 60 * time.Minute
)

var loginLockSteps = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
}

type loginLockEntry struct {
	fails       int
	lockUntil   time.Time
	lockLevel   int
	lastFailAt  time.Time
}

var (
	loginLockMu sync.Mutex
	loginLocks  = map[string]*loginLockEntry{}
)

// loginCheckLock returns (locked, retryAfterSeconds).
// Auto-prunes stale entries whose window elapsed and lock expired.
func loginCheckLock(ip string) (bool, int) {
	loginLockMu.Lock()
	defer loginLockMu.Unlock()
	e := loginLocks[ip]
	if e == nil {
		return false, 0
	}
	now := time.Now()
	// auto reset if window elapsed AND no active lock
	if !e.lastFailAt.IsZero() && now.Sub(e.lastFailAt) > loginFailWindow &&
		(e.lockUntil.IsZero() || !now.Before(e.lockUntil)) {
		delete(loginLocks, ip)
		return false, 0
	}
	if e.lockUntil.IsZero() || !now.Before(e.lockUntil) {
		return false, 0
	}
	remaining := int(e.lockUntil.Sub(now).Seconds())
	if remaining < 1 {
		remaining = 1
	}
	return true, remaining
}

// loginRecordFail increments the fail counter and, on threshold, sets the lock.
// Returns (locked, retryAfterSeconds) AFTER the increment, so the caller can
// emit a 429 + Retry-After when the threshold is just crossed.
func loginRecordFail(ip string) (bool, int) {
	loginLockMu.Lock()
	defer loginLockMu.Unlock()
	e := loginLocks[ip]
	if e == nil {
		e = &loginLockEntry{}
		loginLocks[ip] = e
	}
	e.fails++
	e.lastFailAt = time.Now()
	if e.fails >= loginMaxFailsBeforeLock {
		idx := e.lockLevel
		if idx >= len(loginLockSteps) {
			idx = len(loginLockSteps) - 1
		}
		e.lockUntil = time.Now().Add(loginLockSteps[idx])
		e.lockLevel++
		e.fails = 0
		return true, int(loginLockSteps[idx].Seconds())
	}
	return false, 0
}

// loginRecordSuccess clears the IP's fail history on a successful login.
func loginRecordSuccess(ip string) {
	loginLockMu.Lock()
	defer loginLockMu.Unlock()
	delete(loginLocks, ip)
}
