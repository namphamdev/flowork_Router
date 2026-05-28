package safego

import (
	"sync"
	"testing"
	"time"
)

func TestGo_RunsFunction(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	Go(func() { defer wg.Done() })
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("fn never ran")
	}
}

func TestGo_RecoversPanic(t *testing.T) {
	done := make(chan struct{})
	Go(func() {
		defer close(done)
		panic("boom")
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recover did not let goroutine finish")
	}
}

func TestGoLabel_RecoversPanic(t *testing.T) {
	done := make(chan struct{})
	GoLabel("test", func() {
		defer close(done)
		panic(42)
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("labelled recover did not finish")
	}
}

func TestGo_NoPanicWhenFnReturnsNormally(t *testing.T) {
	count := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		Go(func() {
			defer wg.Done()
			mu.Lock()
			count++
			mu.Unlock()
		})
	}
	wg.Wait()
	if count != 10 {
		t.Fatalf("expected 10 increments, got %d", count)
	}
}
