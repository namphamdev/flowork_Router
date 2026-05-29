// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 9 (Sensors phase 1 webhook subset) DONE. API
//   stable: AuthSource (ValidateSource + ConstantTimeCompare token).
//   Source registry via env `FLOW_ROUTER_SENSOR_<UPPER_ID>_TOKEN`,
//   ID strict regex alphanumeric+dash 2-32 char. Phase 2 (file
//   watcher fsnotify, scheduler cron, multi-source DB registry) →
//   tambah file baru, JANGAN modify ini.
//
// Package sensors — input layer Section 9 (Router roadmap).
//
// Phase 1 SCOPE:
//   Webhook source only. Token-authenticated POST endpoint → forward
//   content ke ingest.Submit pipeline.
//
// Defer phase 2:
//   - File watcher (fsnotify on drop folder)
//   - Scheduler (cron URL fetch)
//   - Multi-source registry persisted di settings DB
//
// SECURITY:
//   - Static source registry via env: `FLOW_ROUTER_SENSOR_<ID>_TOKEN`
//   - Per-source token comparison constant-time (anti timing attack)
//   - Source ID validation: alphanumeric + dash, max 32 char
//
// Source: flowork_Router/roadmap.md Section 9.

package sensors

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// AlgoVersion — sensor layer protocol version.
const AlgoVersion = "v1"

// reSourceID — strict alphanumeric + dash, 2-32 char.
var reSourceID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{1,31}$`)

// ErrUnknownSource — source ID ngga ada di registry / env var ngga set.
var ErrUnknownSource = errors.New("unknown sensor source")

// ErrInvalidToken — token mismatch.
var ErrInvalidToken = errors.New("invalid sensor token")

// ErrInvalidSourceID — source ID format invalid.
var ErrInvalidSourceID = errors.New("invalid source id format")

// ValidateSource — cek source ID format + lookup token dari env.
// Env pattern: `FLOW_ROUTER_SENSOR_<UPPER_ID>_TOKEN` (dash → underscore).
//
// Return token expected. Caller invoke CompareToken untuk match.
func ValidateSource(sourceID string) (expectedToken string, err error) {
	if !reSourceID.MatchString(sourceID) {
		return "", ErrInvalidSourceID
	}
	envKey := "FLOW_ROUTER_SENSOR_" + strings.ToUpper(strings.ReplaceAll(sourceID, "-", "_")) + "_TOKEN"
	expectedToken = os.Getenv(envKey)
	if expectedToken == "" {
		return "", fmt.Errorf("%w: %s (env %s not set)", ErrUnknownSource, sourceID, envKey)
	}
	return expectedToken, nil
}

// CompareToken — constant-time comparison anti timing attack.
func CompareToken(expected, provided string) bool {
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

// AuthSource — combined ValidateSource + CompareToken. Return error
// jelas + opaque (anti info leak).
func AuthSource(sourceID, providedToken string) error {
	expected, err := ValidateSource(sourceID)
	if err != nil {
		// Validation/registry error — caller may log internally.
		return err
	}
	if !CompareToken(expected, providedToken) {
		return ErrInvalidToken
	}
	return nil
}
