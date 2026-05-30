// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package quotalive

import (
	"testing"
	"time"
)

func TestFindCodexReviewRateLimit_TopLevelField(t *testing.T) {
	raw := map[string]any{
		"code_review_rate_limit": map[string]any{
			"primary_window": map[string]any{"used_percent": 25.0},
		},
	}
	got := findCodexReviewRateLimit(raw)
	if got == nil {
		t.Fatal("top-level code_review_rate_limit should be found")
	}
	if _, has := got["primary_window"]; !has {
		t.Fatal("primary_window missing")
	}
}

func TestFindCodexReviewRateLimit_LegacyTopLevelAlias(t *testing.T) {
	raw := map[string]any{
		"review_rate_limit": map[string]any{"primary": map[string]any{"used_percent": 30.0}},
	}
	if findCodexReviewRateLimit(raw) == nil {
		t.Fatal("legacy review_rate_limit alias should be found")
	}
}

func TestFindCodexReviewRateLimit_ByLimitID(t *testing.T) {
	raw := map[string]any{
		"rate_limits_by_limit_id": map[string]any{
			"code_review": map[string]any{"primary_window": map[string]any{"used_percent": 10.0}},
		},
	}
	if findCodexReviewRateLimit(raw) == nil {
		t.Fatal("rate_limits_by_limit_id.code_review should be found")
	}
}

func TestFindCodexReviewRateLimit_AdditionalArray(t *testing.T) {
	raw := map[string]any{
		"additional_rate_limits": []any{
			map[string]any{"limit_name": "completions", "primary_window": map[string]any{}},
			map[string]any{"limit_name": "code_review", "primary_window": map[string]any{"used_percent": 60.0}},
		},
	}
	got := findCodexReviewRateLimit(raw)
	if got == nil {
		t.Fatal("additional_rate_limits[code_review] should be found")
	}
	if got["limit_name"] != "code_review" {
		t.Errorf("wrong entry surfaced: %v", got)
	}
}

func TestFindCodexReviewRateLimit_AdditionalArraySubstring(t *testing.T) {
	raw := map[string]any{
		"additional_rate_limits": []any{
			map[string]any{"limit_name": "advanced_code_review_v2"},
		},
	}
	if findCodexReviewRateLimit(raw) == nil {
		t.Fatal("substring containing 'review' should match")
	}
}

func TestFindCodexReviewRateLimit_AbsentReturnsNil(t *testing.T) {
	raw := map[string]any{
		"rate_limit": map[string]any{"primary_window": map[string]any{"used_percent": 1.0}},
	}
	if findCodexReviewRateLimit(raw) != nil {
		t.Fatal("no review-specific surface should return nil")
	}
}

func TestAppendCodexWindows_PrimaryPlusSecondary(t *testing.T) {
	snap := Snapshot{}
	src := map[string]any{
		"rate_limit": map[string]any{
			"primary_window":   map[string]any{"used_percent": 20.0},
			"secondary_window": map[string]any{"used_percent": 50.0},
		},
	}
	appendCodexWindows(&snap, "", src)
	if len(snap.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(snap.Windows))
	}
	if snap.Windows[0].Label != "session" || snap.Windows[1].Label != "weekly" {
		t.Errorf("labels wrong: %v / %v", snap.Windows[0].Label, snap.Windows[1].Label)
	}
	if snap.Windows[0].Used != 20 || snap.Windows[1].Used != 50 {
		t.Errorf("used values wrong: %v / %v", snap.Windows[0].Used, snap.Windows[1].Used)
	}
}

func TestAppendCodexWindows_ReviewPrefix(t *testing.T) {
	snap := Snapshot{}
	src := map[string]any{
		"primary_window": map[string]any{"used_percent": 10.0},
	}
	appendCodexWindows(&snap, "review", src)
	if len(snap.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(snap.Windows))
	}
	if snap.Windows[0].Label != "review_session" {
		t.Errorf("review prefix not applied: %v", snap.Windows[0].Label)
	}
}

func TestFormatCodexWindow_ClampsPercent(t *testing.T) {
	w := formatCodexWindow("x", map[string]any{"used_percent": 150.0})
	if w.Used != 100 {
		t.Errorf("clamp upper failed: %v", w.Used)
	}
	w = formatCodexWindow("x", map[string]any{"used_percent": -5.0})
	if w.Used != 0 {
		t.Errorf("clamp lower failed: %v", w.Used)
	}
}

func TestFormatCodexWindow_ResetTimeRFC3339(t *testing.T) {
	w := formatCodexWindow("x", map[string]any{
		"used_percent": 5.0,
		"reset_at":     "2026-06-01T00:00:00Z",
	})
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !w.ResetAt.Equal(want) {
		t.Errorf("reset_at RFC3339 parse wrong: %v vs %v", w.ResetAt, want)
	}
}

func TestParseCodexResetTime_UnixSeconds(t *testing.T) {
	got, ok := parseCodexResetTime(float64(1_700_000_000))
	if !ok {
		t.Fatal("seconds epoch should parse")
	}
	if got.Year() != 2023 {
		t.Errorf("unix seconds parsed wrong: %v", got)
	}
}

func TestParseCodexResetTime_UnixMillis(t *testing.T) {
	got, ok := parseCodexResetTime(float64(1_700_000_000_000))
	if !ok {
		t.Fatal("millis epoch should parse")
	}
	if got.Year() != 2023 {
		t.Errorf("millis parsed wrong: %v", got)
	}
}

func TestParseCodexResetTime_NumericString(t *testing.T) {
	got, ok := parseCodexResetTime("1700000000")
	if !ok {
		t.Fatal("numeric string should parse as seconds")
	}
	if got.Year() != 2023 {
		t.Errorf("numeric string parsed wrong: %v", got)
	}
}

func TestParseCodexResetTime_InvalidReturnsFalse(t *testing.T) {
	if _, ok := parseCodexResetTime("not-a-time"); ok {
		t.Fatal("non-RFC3339 non-numeric should not parse")
	}
}

func TestPickCodexPlan_FallbackOrder(t *testing.T) {
	if got := pickCodexPlan(map[string]any{"plan_type": "pro"}); got != "pro" {
		t.Errorf("plan_type wrong: %v", got)
	}
	if got := pickCodexPlan(map[string]any{
		"summary": map[string]any{"plan": "team"},
	}); got != "team" {
		t.Errorf("summary.plan fallback wrong: %v", got)
	}
	if got := pickCodexPlan(map[string]any{}); got != "" {
		t.Errorf("empty should return empty string, got %v", got)
	}
}
