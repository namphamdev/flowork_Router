// Account Fallback Service.

package services

import (
	"strings"
	"time"
)

// BackoffConfig mirrors upstream runtimeConfig BACKOFF_CONFIG.
var BackoffConfig = struct {
	Base     time.Duration // 1st backoff
	Max      time.Duration // cap
	MaxLevel int
}{
	Base:     1 * time.Second,
	Max:      4 * time.Minute,
	MaxLevel: 8,
}

// TransientCooldown is the default unmatched-error cooldown.
const TransientCooldown = 30 * time.Second

// ErrorRule mirrors upstream ERROR_RULES — text match OR status match, with
// optional exponential backoff vs fixed cooldown.
type ErrorRule struct {
	Text       string // substring of lowercased error text
	Status     int    // HTTP status
	Backoff    bool   // when true, escalates via exponential ladder
	CooldownMs int64  // fixed cooldown (ignored when Backoff=true)
}

// Cooldown helpers — named constants so the rule table stays scannable.
var (
	cooldownShort = int64(15 * time.Second / time.Millisecond)
	cooldownMed   = int64(60 * time.Second / time.Millisecond)
	cooldownLong  = int64(5 * time.Minute / time.Millisecond)
)

// ErrorRules — top-down matching, first hit wins. Text rules are checked
// first (more specific than bare status codes) followed by status rules.
// Order matters when the same error matches multiple patterns.
var ErrorRules = []ErrorRule{
	// --- Text-based rules (specific, checked first) ---
	{Text: "no credentials", CooldownMs: cooldownLong},
	{Text: "request not allowed", CooldownMs: cooldownShort},
	{Text: "improperly formed request", CooldownMs: cooldownLong},
	{Text: "rate limit", Backoff: true},
	{Text: "too many requests", Backoff: true},
	{Text: "quota exceeded", Backoff: true},
	{Text: "capacity", Backoff: true},
	{Text: "overloaded", Backoff: true},

	// --- Status-based rules ---
	{Status: 401, CooldownMs: cooldownLong}, // unauthorized — long cool-off (creds likely expired)
	{Status: 402, CooldownMs: cooldownLong}, // payment required — long cool-off
	{Status: 403, CooldownMs: cooldownLong}, // forbidden — long cool-off
	{Status: 404, CooldownMs: cooldownLong}, // not found — model gone, don't retry quickly
	{Status: 429, Backoff: true},            // rate limit — exponential
	{Status: 500, CooldownMs: cooldownShort},
	{Status: 502, CooldownMs: cooldownShort},
	{Status: 503, CooldownMs: cooldownShort},
	{Status: 504, CooldownMs: cooldownShort},
}

// FallbackDecision is the output of CheckFallbackError.
type FallbackDecision struct {
	ShouldFallback  bool
	Cooldown        time.Duration
	NewBackoffLevel int // unchanged when not a backoff rule
}

// GetQuotaCooldown computes exponential cooldown for the given backoff level.
// Level 1 → Base; Level 2 → 2×Base; Level N → 2^(N-1)×Base capped at Max.
func GetQuotaCooldown(backoffLevel int) time.Duration {
	if backoffLevel <= 1 {
		return BackoffConfig.Base
	}
	pow := 1 << (backoffLevel - 1)
	d := BackoffConfig.Base * time.Duration(pow)
	if d > BackoffConfig.Max {
		return BackoffConfig.Max
	}
	return d
}

// CheckFallbackError classifies (status, errorText) and returns the rotation
// decision. backoffLevel is the account's current consecutive-429 counter.
func CheckFallbackError(status int, errorText string, backoffLevel int) FallbackDecision {
	lower := strings.ToLower(errorText)
	for _, rule := range ErrorRules {
		matched := false
		if rule.Text != "" && lower != "" && strings.Contains(lower, rule.Text) {
			matched = true
		}
		if !matched && rule.Status != 0 && rule.Status == status {
			matched = true
		}
		if !matched {
			continue
		}
		if rule.Backoff {
			nl := backoffLevel + 1
			if nl > BackoffConfig.MaxLevel {
				nl = BackoffConfig.MaxLevel
			}
			return FallbackDecision{ShouldFallback: true, Cooldown: GetQuotaCooldown(nl), NewBackoffLevel: nl}
		}
		return FallbackDecision{ShouldFallback: true, Cooldown: time.Duration(rule.CooldownMs) * time.Millisecond, NewBackoffLevel: backoffLevel}
	}
	return FallbackDecision{ShouldFallback: true, Cooldown: TransientCooldown, NewBackoffLevel: backoffLevel}
}

// IsAccountUnavailable returns whether the unavailableUntil moment is still
// in the future.
func IsAccountUnavailable(unavailableUntil time.Time) bool {
	if unavailableUntil.IsZero() {
		return false
	}
	return time.Now().Before(unavailableUntil)
}

// GetUnavailableUntil returns now + cooldown.
func GetUnavailableUntil(cooldown time.Duration) time.Time {
	return time.Now().Add(cooldown)
}

// GetEarliestRateLimitedUntil scans a list of futures and returns the soonest
// one (used to set Retry-After header when ALL accounts are cooling).
func GetEarliestRateLimitedUntil(times []time.Time) (time.Time, bool) {
	var earliest time.Time
	found := false
	now := time.Now()
	for _, t := range times {
		if t.IsZero() || !t.After(now) {
			continue
		}
		if !found || t.Before(earliest) {
			earliest = t
			found = true
		}
	}
	return earliest, found
}
