// Package mesh — http_client.go: shared timeout-bounded HTTP client untuk
// mesh outbound calls (CRDT push, peer connect, gist seed, weight pull, dll).
//
// Sprint 3.5c (BUG-103 fix 2026-05-02): sebelumnya 9 callsite pakai
// http.DefaultClient yang punya Timeout=0 (infinite). Kalau peer mesh hang
// (atau malicious peer delay response), goroutine stuck selamanya →
// goroutine leak → resource exhaustion. Sekarang pakai singleton client
// dengan 10s timeout default.
//
// Note: ngga pakai SafeTransport SSRF guard karena mesh peer URLs DESIGNED
// untuk hit IP private/LAN (peer-to-peer Bitcoin-style). Mesh whitelist via
// peer trust score + Ed25519 signature verification, BUKAN via IP filter.

package mesh

import (
	"net/http"
	"time"
)

// meshClient — global timeout-bounded client untuk semua mesh outbound
// calls. Reuse connection pool default Transport (efficient for repeated
// calls ke peer yang sama).
var meshClient = &http.Client{Timeout: 10 * time.Second}
