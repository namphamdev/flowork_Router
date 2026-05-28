package streamutil

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// blockingReader returns a fixed number of chunks separated by a sleep, then
// hangs indefinitely until Close is called. Used to simulate a stalled
// upstream where the connection stays open but bytes stop arriving.
type blockingReader struct {
	chunks   []string
	gap      time.Duration
	mu       sync.Mutex
	idx      int
	closed   chan struct{}
	closedMu sync.Mutex
	isClosed bool
}

func newBlockingReader(gap time.Duration, chunks ...string) *blockingReader {
	return &blockingReader{
		chunks: chunks,
		gap:    gap,
		closed: make(chan struct{}),
	}
}

func (b *blockingReader) Read(p []byte) (int, error) {
	b.mu.Lock()
	if b.idx >= len(b.chunks) {
		b.mu.Unlock()
		// Hang forever (or until Close is called).
		select {
		case <-b.closed:
			return 0, io.EOF
		}
	}
	chunk := b.chunks[b.idx]
	b.idx++
	b.mu.Unlock()

	if b.gap > 0 && b.idx > 1 {
		select {
		case <-time.After(b.gap):
		case <-b.closed:
			return 0, io.EOF
		}
	}
	n := copy(p, chunk)
	return n, nil
}

func (b *blockingReader) Close() error {
	b.closedMu.Lock()
	defer b.closedMu.Unlock()
	if b.isClosed {
		return nil
	}
	b.isClosed = true
	close(b.closed)
	return nil
}

func TestStallReader_PassesThroughWhenDataFlows(t *testing.T) {
	src := io.NopCloser(strings.NewReader("hello world"))
	sr := NewStallReader(src, 100*time.Millisecond)
	defer sr.Close()
	got, err := io.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Fatalf("data mismatch: %q", got)
	}
}

func TestStallReader_FiresOnInactivity(t *testing.T) {
	// 1st chunk arrives immediately, 2nd is delayed past the timeout.
	src := newBlockingReader(150*time.Millisecond, "first", "second")
	sr := NewStallReader(src, 50*time.Millisecond)
	defer sr.Close()

	buf := make([]byte, 16)
	n, err := sr.Read(buf) // first chunk succeeds
	if err != nil || n == 0 {
		t.Fatalf("first read failed: n=%d err=%v", n, err)
	}
	// Second read should hit the stall.
	_, err = sr.Read(buf)
	if !errors.Is(err, ErrStreamStall) {
		t.Fatalf("expected ErrStreamStall, got %v", err)
	}
	if !sr.HasStalled() {
		t.Fatal("HasStalled should report true after firing")
	}
}

func TestStallReader_SubsequentReadsAfterStall(t *testing.T) {
	src := newBlockingReader(100*time.Millisecond, "a")
	sr := NewStallReader(src, 30*time.Millisecond)
	defer sr.Close()

	buf := make([]byte, 4)
	_, _ = sr.Read(buf) // first chunk
	_, firstErr := sr.Read(buf)
	if !errors.Is(firstErr, ErrStreamStall) {
		t.Fatalf("expected stall on second read, got %v", firstErr)
	}
	// Subsequent reads continue to return ErrStreamStall WITHOUT touching
	// the closed source.
	_, secondErr := sr.Read(buf)
	if !errors.Is(secondErr, ErrStreamStall) {
		t.Fatalf("expected sticky stall, got %v", secondErr)
	}
}

func TestStallReader_DisabledWhenTimeoutZero(t *testing.T) {
	src := newBlockingReader(80*time.Millisecond, "alpha", "beta")
	sr := NewStallReader(src, 0)
	defer sr.Close()
	buf := make([]byte, 8)
	if _, err := sr.Read(buf); err != nil {
		t.Fatalf("first read err: %v", err)
	}
	// Without stall detection, the slow read just blocks and eventually
	// returns the next chunk.
	if _, err := sr.Read(buf); err != nil {
		t.Fatalf("second read err: %v", err)
	}
}

func TestStallReader_CloseIsIdempotent(t *testing.T) {
	src := io.NopCloser(strings.NewReader("x"))
	sr := NewStallReader(src, 10*time.Second)
	if err := sr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sr.Close(); err != nil {
		t.Fatal("second Close must not error")
	}
}

func TestStallReader_StallClosesUnderlyingSource(t *testing.T) {
	src := newBlockingReader(100*time.Millisecond, "boot")
	sr := NewStallReader(src, 20*time.Millisecond)
	buf := make([]byte, 8)
	_, _ = sr.Read(buf) // first
	_, _ = sr.Read(buf) // stall
	src.closedMu.Lock()
	defer src.closedMu.Unlock()
	if !src.isClosed {
		t.Fatal("stall watchdog must Close the underlying source")
	}
}

func TestStallReader_NormalEOFNotMisreportedAsStall(t *testing.T) {
	src := io.NopCloser(bytes.NewReader([]byte("done")))
	sr := NewStallReader(src, 200*time.Millisecond)
	defer sr.Close()
	buf := make([]byte, 4)
	_, _ = sr.Read(buf)         // 4 bytes
	_, err := sr.Read(buf)      // EOF
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
	if sr.HasStalled() {
		t.Fatal("HasStalled should remain false on natural EOF")
	}
}

func TestDefaultStallTimeout_MatchesUpstreamReference(t *testing.T) {
	if DefaultStallTimeout != 35*time.Second {
		t.Fatalf("DefaultStallTimeout drifted from upstream 35s: %v", DefaultStallTimeout)
	}
}
