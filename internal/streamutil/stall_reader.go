// Stream-stall guard. Wraps an io.ReadCloser so reads abort when no bytes
// arrive within a configurable timeout window. Without this guard, a
// silently-stuck upstream keeps the client connection open indefinitely —
// the user sees a "spinning forever" UI and the goroutine never exits.
//
// Use:
//   src := stallReader(upstream.Body, 35*time.Second)
//   defer src.Close()
//   io.Copy(dst, src)
//
// On stall, the next Read returns ErrStreamStall and triggers Close on the
// underlying source so the upstream goroutine unwinds too.

package streamutil

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultStallTimeout matches the upstream reference: 35 seconds without a
// chunk is treated as a stall.
const DefaultStallTimeout = 35 * time.Second

// ErrStreamStall is returned by Read once the inactivity timeout fires.
var ErrStreamStall = errors.New("stream stalled: no data within timeout")

// StallReader wraps an io.ReadCloser with an inactivity deadline. Each call
// to Read resets the timer. When the timer fires before the next Read
// returns, subsequent reads return ErrStreamStall and the underlying
// source is closed.
type StallReader struct {
	src     io.ReadCloser
	timeout time.Duration
	mu      sync.Mutex
	stalled atomic.Bool
	closed  atomic.Bool
	cancel  chan struct{} // closed when Close() is called; halts the watchdog
}

// NewStallReader wraps src with the given inactivity timeout. A non-positive
// timeout disables stall detection (Read passes through unchanged).
func NewStallReader(src io.ReadCloser, timeout time.Duration) *StallReader {
	return &StallReader{
		src:     src,
		timeout: timeout,
		cancel:  make(chan struct{}),
	}
}

// Read implements io.Reader. Each successful read resets the watchdog.
// After ErrStreamStall fires once, subsequent reads keep returning it
// without touching the (closed) source.
func (r *StallReader) Read(p []byte) (int, error) {
	if r.stalled.Load() {
		return 0, ErrStreamStall
	}
	if r.timeout <= 0 {
		return r.src.Read(p)
	}

	// Watchdog: AfterFunc schedules the close callback on a runtime goroutine
	// that exits as soon as it fires (or is stopped). Stop() races safely with
	// the callback — if the timer already fired, Stop returns false and the
	// stalled flag has already been set.
	timer := time.AfterFunc(r.timeout, func() {
		if !r.stalled.CompareAndSwap(false, true) {
			return
		}
		_ = r.src.Close()
	})

	n, err := r.src.Read(p)
	timer.Stop()

	if r.stalled.Load() {
		return n, ErrStreamStall
	}
	return n, err
}

// Close closes the underlying source. Safe to call multiple times.
func (r *StallReader) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(r.cancel)
	return r.src.Close()
}

// HasStalled reports whether the stall trigger fired. Useful in unit tests
// to distinguish a genuine EOF from a stall-induced close.
func (r *StallReader) HasStalled() bool { return r.stalled.Load() }
