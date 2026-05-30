// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package services

import (
	"testing"
	"time"
)

func TestErrorRule_NoCredentials_LongCooldown(t *testing.T) {
	d := CheckFallbackError(0, "Provider error: no credentials available", 0)
	if d.Cooldown < 4*time.Minute {
		t.Fatalf("\"no credentials\" must trigger long cooldown, got %v", d.Cooldown)
	}
}

func TestErrorRule_RequestNotAllowed_ShortCooldown(t *testing.T) {
	d := CheckFallbackError(0, "Upstream said request not allowed for this model", 0)
	if d.Cooldown != 15*time.Second {
		t.Fatalf("\"request not allowed\" should be 15s, got %v", d.Cooldown)
	}
}

func TestErrorRule_ImproperlyFormed_LongCooldown(t *testing.T) {
	d := CheckFallbackError(0, "Improperly formed request body", 0)
	if d.Cooldown < 4*time.Minute {
		t.Fatalf("\"improperly formed\" must be long cooldown, got %v", d.Cooldown)
	}
}

func TestErrorRule_Capacity_Backoff(t *testing.T) {
	d := CheckFallbackError(0, "service at capacity", 0)
	if d.NewBackoffLevel != 1 {
		t.Fatalf("capacity should escalate backoff, got %+v", d)
	}
}

func TestErrorRule_Overloaded_Backoff(t *testing.T) {
	d := CheckFallbackError(0, "Server overloaded — retry later", 0)
	if d.NewBackoffLevel != 1 {
		t.Fatalf("overloaded should escalate backoff, got %+v", d)
	}
}

func TestErrorRule_402_LongCooldown(t *testing.T) {
	d := CheckFallbackError(402, "", 0)
	if d.Cooldown < 4*time.Minute {
		t.Fatalf("402 should be long cooldown, got %v", d.Cooldown)
	}
}

func TestErrorRule_404_LongCooldown(t *testing.T) {
	d := CheckFallbackError(404, "model not found", 0)
	if d.Cooldown < 4*time.Minute {
		t.Fatalf("404 should be long cooldown, got %v", d.Cooldown)
	}
}

func TestErrorRule_TextRulesWinOverStatusRules(t *testing.T) {
	// Status 500 + text "capacity" — text rule should win (backoff, not 15s).
	d := CheckFallbackError(500, "internal server error: capacity reached", 0)
	if d.NewBackoffLevel != 1 {
		t.Fatalf("capacity text rule must win over status 500, got %+v", d)
	}
}

func TestErrorRule_TotalCount(t *testing.T) {
	want := 17 // 8 text + 9 status (incl 429)
	if got := len(ErrorRules); got != want {
		t.Fatalf("ErrorRules count drifted: got %d, want %d", got, want)
	}
}
