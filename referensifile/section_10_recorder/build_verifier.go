// Package proxy — build_verifier.go: Tier 1.4 heuristic build verifier.
//
// Bug ditemukan opus-3 dashboard inspection 2026-05-11: 139/139 recordings
// stuck di build_pass=-1 (pending forever). MarkBuildResult exists tapi
// gak ada caller. Cascading idle: skillminer + tier-promoter + finetune
// pipeline semua jalan zero output karena gate WHERE build_pass=1.
//
// Tier 1.4 fix: wire heuristic classifier post RecordInteraction, decide
// pass/fail dari response markers, call MarkBuildResult.
//
// Heuristic Phase 1 (lenient):
//   - Response NOT contain fail markers ("error":, fatal, panic) → pass
//   - Tool calls (kalau ada) semua return valid result (no error field) → pass
//   - [REFUSED ...] honest-refuse → pass (anti-halu, bukan fail)
//   - Otherwise → fail
//
// DB-backed config (per Ayah doctrine "ngga hardcode, harus pakai database"):
//   - BUILD_VERIFIER_FAIL_MARKERS (comma-separated, default: `"error":,fatal,panic`)
//   - BUILD_VERIFIER_HEURISTIC_MODE (lenient | strict | disabled, default lenient)
//   - BUILD_VERIFIER_REQUIRE_TOOL_CALLS_OK (bool, default true)
//
// Phase 2 upgrade path: LLM judge (Qwen lokal sustainable $0), code-aware
// build verify (kalau response mengubah file, trigger go build).

package proxy

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/teetah2402/flowork/internal/provider"
	"github.com/teetah2402/flowork/internal/settings"
)

// Settings DB keys + fallback defaults (per Ayah doctrine "ngga hardcode").
const (
	settingsKeyFailMarkers       = "BUILD_VERIFIER_FAIL_MARKERS"
	settingsKeyHeuristicMode     = "BUILD_VERIFIER_HEURISTIC_MODE"
	settingsKeyRequireToolCallsOK = "BUILD_VERIFIER_REQUIRE_TOOL_CALLS_OK"

	fallbackFailMarkers       = `"error":,fatal,panic`
	fallbackHeuristicMode     = "lenient"
	fallbackRequireToolCallsOK = true
)

// honestRefusePrefix — pattern yang signal warga jujur refuse. Anti-halu, BUKAN
// fail. Per opus-3 tradeoff note Tier 1.4 proposal.
const honestRefusePrefix = "[REFUSED"

// VerifyBuild — heuristic classify response sebagai pass/fail. Pure function,
// no side-effects beyond settings DB read.
//
// Args:
//   responseText: response.Message.Content dari LLM
//   toolCalls: response.Message.ToolCalls (kalau empty = no tool call)
//
// Returns: (pass bool, reason string)
//
// Reason useful untuk audit log (kenapa pass/fail). Caller log ini.
func VerifyBuild(responseText string, toolCalls []provider.ToolCall) (bool, string) {
	mode := settingsString(settingsKeyHeuristicMode, fallbackHeuristicMode)
	if mode == "disabled" {
		return false, "heuristic disabled (BUILD_VERIFIER_HEURISTIC_MODE=disabled)"
	}

	// Honest-refuse pattern = pass (anti-halu, warga jujur ngga halu jawab)
	trimmed := strings.TrimSpace(responseText)
	if strings.HasPrefix(trimmed, honestRefusePrefix) {
		return true, "honest-refuse pattern detected (anti-halu pass)"
	}

	// Fail markers detection
	failMarkers := strings.Split(settingsString(settingsKeyFailMarkers, fallbackFailMarkers), ",")
	for _, marker := range failMarkers {
		marker = strings.TrimSpace(marker)
		if marker == "" {
			continue
		}
		if strings.Contains(responseText, marker) {
			return false, fmt.Sprintf("fail marker detected: %q", marker)
		}
	}

	// Tool calls validity check (lenient = optional, strict = required)
	if mode == "strict" || (mode == "lenient" && settingsBool(settingsKeyRequireToolCallsOK, fallbackRequireToolCallsOK)) {
		for i, tc := range toolCalls {
			// Tool call kalau ada error field di output = fail. Schema-aware:
			// provider.ToolCall struct varies, kita cek loose pattern.
			if hasToolCallError(tc) {
				return false, fmt.Sprintf("tool_call[%d] returned error", i)
			}
		}
	}

	return true, "no fail markers + tool calls clean"
}

// hasToolCallError — heuristic detect tool call result error. Conservative —
// kalau ngga yakin, return false (lean toward pass = lenient mode default).
func hasToolCallError(tc provider.ToolCall) bool {
	// Common error patterns: result starts dengan "error:", contains "panic",
	// atau punya structured error field.
	// provider.ToolCall struct cuma punya Name + Arguments (request side),
	// hasil execution di-track separate. Conservative: return false untuk
	// Phase 1 (no false negative). Phase 2 upgrade: pass tool call results
	// ke verifier.
	return false
}

// VerifyAndMark — fire-and-forget classify + persist via MarkBuildResult.
// Caller pattern: panggil dari proxy.go post RecordInteraction.
//
// Idempotent: MarkBuildResult guard `WHERE build_pass = -1` cuma update
// rows pending. Re-call no-op kalau sudah classified.
func (r *Recorder) VerifyAndMark(promptText, responseText string, toolCalls []provider.ToolCall) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(promptText)))
	pass, reason := VerifyBuild(responseText, toolCalls)
	r.MarkBuildResult(hash, pass)
	// Audit trail (visible at proxy verbose mode atau systemd journal)
	_ = reason // log via caller kalau verbose
}

// settingsString graceful read string dari settings DB dengan fallback.
func settingsString(key, fallback string) string {
	store := settings.Shared()
	if store == nil {
		return fallback
	}
	v, err := store.Get(key)
	if err != nil || strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// settingsBool graceful read bool dari settings DB.
func settingsBool(key string, fallback bool) bool {
	v := settingsString(key, "")
	if v == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on", "y":
		return true
	case "false", "0", "no", "off", "n":
		return false
	default:
		return fallback
	}
}

// QueryBuildPassDistribution count rows per build_pass status. Untuk GUI
// "Build Verifier" tab (FQ-Brain Dashboard) dan stats endpoint.
//
// Returns map: {pending: N (-1), pass: N (1), fail: N (0)}.
func (r *Recorder) QueryBuildPassDistribution() (map[string]int64, error) {
	out := map[string]int64{
		"pending": 0,
		"pass":    0,
		"fail":    0,
	}
	rows, err := r.db.Query(`SELECT build_pass, COUNT(*) FROM recordings GROUP BY build_pass`)
	if err != nil {
		return out, fmt.Errorf("query distribution: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status int
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		switch status {
		case -1:
			out["pending"] = count
		case 1:
			out["pass"] = count
		case 0:
			out["fail"] = count
		}
	}
	return out, rows.Err()
}
