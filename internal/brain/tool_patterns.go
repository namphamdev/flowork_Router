// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 6 (Tool learner phase 1) DONE. API stable:
//   LearnPattern (atomic UPSERT via ON CONFLICT DO UPDATE RETURNING
//   amplitude), SuggestTools (LIKE substring rank by amplitude DESC,
//   cap 10 anti over-prompt), CountToolPatterns. Pattern extraction
//   currently substring match — phase 2 add n-gram / embedding cluster
//   via new function di file lain, JANGAN modify ini.
//
// tool_patterns.go — Section 6 roadmap: Tool learner.
//
// PURPOSE:
//   Learn pattern `trigger → tool` dari interaction history. Upsert ke
//   `tool_patterns` table (sudah ada schema). Suggest tool berdasarkan
//   trigger query — ranked by amplitude (success_count / (success + fail)).
//
// SEMANTIC:
//   - LearnPattern(trigger, tool, success): UPSERT by (trigger_pattern,
//     tool_name) UNIQUE. success=true → success_count++, else fail_count++.
//     Recompute amplitude after update.
//   - SuggestTools(trigger, limit): match trigger via LIKE prefix search,
//     rank by amplitude DESC. Default limit 5, max 10 (anti over-prompt).
//
// Source: flowork_Router/roadmap.md Section 6.

package brain

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// AlgoVersion — pattern extraction algorithm version.
const ToolLearnerAlgoVersion = "v1"

// ToolPattern — satu row di tool_patterns.
type ToolPattern struct {
	ID             int64   `json:"id"`
	TriggerPattern string  `json:"trigger_pattern"`
	ToolName       string  `json:"tool_name"`
	SuccessCount   int64   `json:"success_count"`
	FailCount      int64   `json:"fail_count"`
	Amplitude      float64 `json:"amplitude"`
}

// LearnPattern — upsert pattern. Success increment success_count, else
// fail_count. Amplitude recomputed = success / (success + fail), clamped
// to [0.0, 1.0]. Return resulting amplitude.
//
// Trigger + tool name validate: required, max 256 char each.
func LearnPattern(ctx context.Context, trigger, toolName string, success bool) (float64, error) {
	trigger = strings.TrimSpace(trigger)
	toolName = strings.TrimSpace(toolName)
	if trigger == "" || toolName == "" {
		return 0, fmt.Errorf("trigger + tool_name required")
	}
	const maxLen = 256
	if len(trigger) > maxLen {
		trigger = trigger[:maxLen]
	}
	if len(toolName) > maxLen {
		toolName = toolName[:maxLen]
	}

	db, err := OpenRW()
	if err != nil {
		return 0, err
	}

	// Atomic UPSERT: insert atau increment counter sesuai outcome.
	// Amplitude recomputed via subquery — single statement.
	successDelta := 0
	failDelta := 0
	if success {
		successDelta = 1
	} else {
		failDelta = 1
	}

	var amplitude float64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO tool_patterns(trigger_pattern, tool_name, success_count, fail_count, amplitude)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(trigger_pattern, tool_name) DO UPDATE SET
		     success_count = tool_patterns.success_count + excluded.success_count,
		     fail_count    = tool_patterns.fail_count + excluded.fail_count,
		     amplitude     = CAST(tool_patterns.success_count + excluded.success_count AS REAL) /
		                     MAX(1, tool_patterns.success_count + tool_patterns.fail_count + excluded.success_count + excluded.fail_count),
		     deleted_at    = NULL,
		     deleted_by    = NULL
		 RETURNING amplitude`,
		trigger, toolName, successDelta, failDelta, computeInitialAmplitude(successDelta, failDelta),
	).Scan(&amplitude); err != nil {
		return 0, fmt.Errorf("upsert pattern: %w", err)
	}
	return amplitude, nil
}

// computeInitialAmplitude — formula success / (success + fail), default 0.5
// kalau total=0.
func computeInitialAmplitude(success, fail int) float64 {
	total := success + fail
	if total == 0 {
		return 0.5
	}
	return float64(success) / float64(total)
}

// SuggestTools — match trigger via LIKE prefix + amplitude rank. Return top-K
// (limit, max 10). Anti over-prompt: kalau caller butuh lebih banyak, pakai
// dedicated browse endpoint future.
func SuggestTools(ctx context.Context, trigger string, limit int) ([]ToolPattern, error) {
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		return nil, fmt.Errorf("trigger required")
	}
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	db, err := OpenRW()
	if err != nil {
		return nil, err
	}
	// Pattern: substring match (LIKE '%trigger%'), exclude soft-deleted,
	// rank by amplitude DESC then success_count DESC (tiebreaker).
	rows, qerr := db.QueryContext(ctx,
		`SELECT id, trigger_pattern, tool_name, success_count, fail_count, amplitude
		 FROM tool_patterns
		 WHERE deleted_at IS NULL
		   AND trigger_pattern LIKE ?
		 ORDER BY amplitude DESC, success_count DESC
		 LIMIT ?`,
		"%"+trigger+"%", limit,
	)
	if qerr != nil {
		return nil, fmt.Errorf("query tool patterns: %w", qerr)
	}
	defer rows.Close()

	var out []ToolPattern
	for rows.Next() {
		var p ToolPattern
		if err := rows.Scan(&p.ID, &p.TriggerPattern, &p.ToolName,
			&p.SuccessCount, &p.FailCount, &p.Amplitude); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CountToolPatterns — non-deleted count.
func CountToolPatterns(ctx context.Context) (int64, error) {
	db, err := OpenRW()
	if err != nil {
		return 0, err
	}
	var n int64
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tool_patterns WHERE deleted_at IS NULL`,
	).Scan(&n); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return n, nil
}
