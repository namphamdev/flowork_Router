// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package helpers

import "testing"

func TestAdjustMaxTokens_BasicNoToolsNoThinking(t *testing.T) {
	if got := AdjustMaxTokens(8192, false, 0); got != 8192 {
		t.Fatalf("plain request must pass through: got %d", got)
	}
}

func TestAdjustMaxTokens_ZeroInputUsesDefault(t *testing.T) {
	if got := AdjustMaxTokens(0, false, 0); got != DefaultMaxTokens {
		t.Fatalf("zero input must fall back to DefaultMaxTokens (%d), got %d", DefaultMaxTokens, got)
	}
}

func TestAdjustMaxTokens_ToolsBumpsToFloor(t *testing.T) {
	if got := AdjustMaxTokens(4096, true, 0); got != MinMaxTokensWithTools {
		t.Fatalf("tools should bump to %d, got %d", MinMaxTokensWithTools, got)
	}
}

func TestAdjustMaxTokens_ToolsLeaveAlreadyLargeAlone(t *testing.T) {
	if got := AdjustMaxTokens(64000, true, 0); got != 64000 {
		t.Fatalf("when caller already provides ≥floor, leave alone: got %d", got)
	}
}

func TestAdjustMaxTokens_ThinkingBumpsAboveBudget(t *testing.T) {
	if got := AdjustMaxTokens(4096, false, 16000); got != 16000+1024 {
		t.Fatalf("thinking budget should bump max to budget+1024, got %d", got)
	}
}

func TestAdjustMaxTokens_ThinkingBudgetExceededWhenEqual(t *testing.T) {
	// max_tokens == budget should still bump (API requires strictly >).
	if got := AdjustMaxTokens(16000, false, 16000); got != 16000+1024 {
		t.Fatalf("equal-to-budget must bump, got %d", got)
	}
}

func TestAdjustMaxTokens_ThinkingLeavesLargerAlone(t *testing.T) {
	if got := AdjustMaxTokens(64000, false, 16000); got != 64000 {
		t.Fatalf("max already > budget should be untouched, got %d", got)
	}
}

func TestAdjustMaxTokens_ToolsAndThinkingCombined(t *testing.T) {
	// hasTools=true → floor 32000, then budget=40000 → bump to 41024.
	if got := AdjustMaxTokens(4096, true, 40000); got != 40000+1024 {
		t.Fatalf("combined: expected %d, got %d", 41024, got)
	}
}

func TestAdjustMaxTokens_BothRulesNoEffectWhenAlreadyAboveBoth(t *testing.T) {
	if got := AdjustMaxTokens(100000, true, 50000); got != 100000 {
		t.Fatalf("large input should pass through: got %d", got)
	}
}
