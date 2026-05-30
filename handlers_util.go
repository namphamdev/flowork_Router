// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — HTTP handler. Method validation + JSON response + error handling per Router convention.

// Handler utilities + globals.

package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"
)

var (
	processStartedAt  = time.Now().UTC()
	shutdownTriggerCh chan struct{}
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// constantTimeEqualString compares two strings in constant time, suitable for
// CSRF tokens and other short secrets where length is not itself sensitive.
func constantTimeEqualString(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
