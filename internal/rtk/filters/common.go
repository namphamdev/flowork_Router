// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/rtk/filters package — audit pass surface review.

// Common helpers shared by filters: pre-compiled regex helpers + itoa.
package filters

import (
	"regexp"
	"strconv"
)

func mustCompile(p string) *regexp.Regexp { return regexp.MustCompile(p) }

func itoa(n int) string { return strconv.Itoa(n) }
