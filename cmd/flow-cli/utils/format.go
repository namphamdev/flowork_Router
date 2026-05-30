// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Formatters for currency, duration, and JSON pretty-print.
package utils

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// USD formats a float as "$1.23" / "$0.0042" depending on magnitude.
func USD(v float64) string {
	switch {
	case v >= 1:
		return fmt.Sprintf("$%.2f", v)
	case v >= 0.01:
		return fmt.Sprintf("$%.4f", v)
	default:
		return fmt.Sprintf("$%.6f", v)
	}
}

// Duration formats a millisecond count as a human-friendly string.
func Duration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	d := time.Duration(ms) * time.Millisecond
	return d.Truncate(10 * time.Millisecond).String()
}

// PrettyJSON renders v as 2-space indented JSON. Falls back to fmt.Sprintf on
// marshal error so the call site is one-liner safe.
func PrettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// Truncate cuts s at n characters, appending "…" when cut.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Joiner returns a comma-separated string from a string slice (empty when nil).
func Joiner(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.Join(items, ", ")
}
