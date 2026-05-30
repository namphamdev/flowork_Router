// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Goroutine launcher with a panic recovery net.
//
// Fire-and-forget background work (metrics, logging, MITM capture, pipe
// drainers) inherits the default Go behaviour: an unrecovered panic in any
// goroutine crashes the entire process. safego wraps the work in a deferred
// recover and logs the stack so a bug in a background task can't tear down
// the whole router.
//
// Use Go(fn) for the common case. Pass a label string when you want the
// recovery log to point at a specific call site.

package safego

import (
	"log"
	"runtime/debug"
)

// Go runs fn in a new goroutine with panic recovery.
func Go(fn func()) {
	GoLabel("", fn)
}

// GoLabel is Go with a custom label printed on recovery — useful when a
// generic stack trace isn't enough to find the offending site.
func GoLabel(label string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if label == "" {
					log.Printf("safego: recovered panic: %v\n%s", r, debug.Stack())
					return
				}
				log.Printf("safego[%s]: recovered panic: %v\n%s", label, r, debug.Stack())
			}
		}()
		fn()
	}()
}
