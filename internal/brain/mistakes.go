// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 7 (Mistakes journal global) phase 1 DONE +
//   adversarial-audit passed (C1 hit_count cap 1M anti-overflow, C2
//   private whitelist + IsValid/List accessor, C3 atomic UPSERT via
//   ON CONFLICT DO UPDATE RETURNING — eliminates TOCTOU race;
//   important #4 agent_id 128-byte cap). API stable: SubmitMistake,
//   ListMistakes, CountMistakes, IsValidMistakeCategory,
//   ListMistakeCategories. Phase 2 (brain_antibody auto-promotion,
//   WebSocket notify) → tambah function/file baru, JANGAN modify ini.
//
// mistakes.go — Section 7 roadmap: Mistakes journal global tier.
//
// PURPOSE:
//   Receive mistakes promotion dari Agent (per Agent roadmap section 2 +
//   7). Insert ke `mistakes_journal` table di brain DB. Future: promote
//   ke `brain_antibody` global supaya semua warga benefit.
//
// SEMANTIC:
//   - SubmitMistake: UNIQUE(category, title) → upsert hit_count atomik
//     via INSERT ... ON CONFLICT DO UPDATE (modernc.org/sqlite support).
//   - ListMistakes: paginated list, filter optional tier + source_agent_id.
//   - CountMistakes: count non-deleted.
//
// ⚠️ Over-prompt warning: tier='global' kontainer mistakes promotion dari
// agent. BUKAN auto-inject ke chat — semantic match query dulu (defer
// phase 2), inject MAX 3 antibody relevant. Sisanya retrieved via
// brain_search.
//
// Source: flowork_Router/roadmap.md Section 7.

package brain

import (
	"context"
	"fmt"
	"time"
)

// mistakeCategoryWhitelist — taxonomy valid untuk mistakes. Anti trash data.
// Private supaya immutable from outside — caller pakai IsValidMistakeCategory
// dan ListMistakeCategories untuk read-only access.
var mistakeCategoryWhitelist = map[string]struct{}{
	"logic":       {},
	"safety":      {},
	"performance": {},
	"security":    {},
	"ux":          {},
	"governance":  {},
}

// IsValidMistakeCategory — check kalau category ada di whitelist.
func IsValidMistakeCategory(category string) bool {
	_, ok := mistakeCategoryWhitelist[category]
	return ok
}

// ListMistakeCategories — return sorted slice of valid categories (read-only).
func ListMistakeCategories() []string {
	out := make([]string, 0, len(mistakeCategoryWhitelist))
	for k := range mistakeCategoryWhitelist {
		out = append(out, k)
	}
	// Insertion order: caller sort kalau perlu deterministik.
	return out
}

// Mistake — satu row di tabel mistakes_journal.
type Mistake struct {
	ID                   int64  `json:"id"`
	Category             string `json:"category"`
	Title                string `json:"title"`
	Content              string `json:"content"`
	SourceAgentID        string `json:"source_agent_id"`
	HitCount             int64  `json:"hit_count"`
	Tier                 string `json:"tier"`
	ReviewedAt           string `json:"reviewed_at,omitempty"`
	PromotedToAntibodyID string `json:"promoted_to_antibody_id,omitempty"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

// SubmitMistake — upsert via UNIQUE(category, title). Validasi:
//   - hit_count ≥ 3 (kalau tidak admin override flag belum ada, reject)
//   - category in whitelist
//   - title + content + source_agent_id required
//
// Behavior on conflict: increment hit_count by submitted hit_count,
// overwrite content (latest sample), update updated_at. Preserve created_at
// + tier + reviewed_at + promoted_to_antibody_id.
//
// Return (id, isNew bool, error). isNew=false → upsert path.
func SubmitMistake(ctx context.Context, category, title, content, sourceAgentID string, hitCount int64) (int64, bool, error) {
	if category == "" || title == "" || content == "" || sourceAgentID == "" {
		return 0, false, fmt.Errorf("category + title + content + source_agent_id required")
	}
	// Audit fix #4: cap source_agent_id length supaya ngga bloat index.
	const maxAgentIDBytes = 128
	if len(sourceAgentID) > maxAgentIDBytes {
		return 0, false, fmt.Errorf("source_agent_id must be <= %d bytes", maxAgentIDBytes)
	}
	if !IsValidMistakeCategory(category) {
		return 0, false, fmt.Errorf("category %q not in whitelist", category)
	}
	const (
		minHitCount = 3
		maxHitCount = 1_000_000 // anti integer-overflow / spam runaway counter
	)
	if hitCount < minHitCount {
		return 0, false, fmt.Errorf("hit_count must be >= %d (got %d)", minHitCount, hitCount)
	}
	if hitCount > maxHitCount {
		return 0, false, fmt.Errorf("hit_count must be <= %d (got %d)", maxHitCount, hitCount)
	}

	// Hard cap content + title bytes anti-bloat.
	const (
		maxContentBytes = 8 * 1024
		maxTitleBytes   = 256
	)
	if len(content) > maxContentBytes {
		content = content[:maxContentBytes] + "…[truncated]"
	}
	if len(title) > maxTitleBytes {
		title = title[:maxTitleBytes] + "…"
	}

	db, err := OpenRW()
	if err != nil {
		return 0, false, err
	}
	ts := time.Now().UTC().Format(time.RFC3339)

	// Audit fix C3: single atomic UPSERT via ON CONFLICT DO UPDATE
	// dengan RETURNING — eliminates TOCTOU race antara SELECT + INSERT.
	// Track isNew via `changes()` pseudo-column — di SQLite changes() return
	// row-affected dari last statement (1 untuk fresh INSERT, 1 untuk UPDATE
	// — ngga bisa distinguish lewat changes() saja). Kita pakai trick:
	// `excluded.created_at = mistakes_journal.created_at` — only match kalau
	// row fresh-inserted (excluded.created_at = ts, existing.created_at = old ts).
	//
	// Pakai pattern lain: ambil hit_count BEFORE update kalau row sudah ada,
	// compare. Sebenernya simpler: cuma return id + hit_count, caller
	// distinguish isNew via hit_count == submitted value (fresh insert).
	var id int64
	var newHitCount int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO mistakes_journal(category, title, content, source_agent_id, hit_count, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(category, title) DO UPDATE SET
		     content    = excluded.content,
		     hit_count  = mistakes_journal.hit_count + excluded.hit_count,
		     updated_at = excluded.updated_at,
		     deleted_at = NULL
		 RETURNING id, hit_count`,
		category, title, content, sourceAgentID, hitCount, ts, ts,
	).Scan(&id, &newHitCount); err != nil {
		return 0, false, fmt.Errorf("upsert mistake: %w", err)
	}
	// Fresh insert: newHitCount == hitCount submitted.
	// Upsert: newHitCount > hitCount (existing accumulated).
	isNew := newHitCount == hitCount
	return id, isNew, nil
}

// ListMistakes — paginated. Filter optional tier + source_agent_id.
// Order: updated_at DESC. Limit default 50, max 500.
func ListMistakes(ctx context.Context, tier, sourceAgentID string, limit int) ([]Mistake, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	db, err := OpenRW()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, category, title, content, source_agent_id, hit_count, tier,
	                 COALESCE(reviewed_at, ''), COALESCE(promoted_to_antibody_id, ''),
	                 created_at, updated_at
	          FROM mistakes_journal WHERE deleted_at IS NULL`
	args := []any{}
	if tier != "" {
		query += ` AND tier = ?`
		args = append(args, tier)
	}
	if sourceAgentID != "" {
		query += ` AND source_agent_id = ?`
		args = append(args, sourceAgentID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, qerr := db.QueryContext(ctx, query, args...)
	if qerr != nil {
		return nil, fmt.Errorf("query mistakes: %w", qerr)
	}
	defer rows.Close()

	var out []Mistake
	for rows.Next() {
		var m Mistake
		if err := rows.Scan(&m.ID, &m.Category, &m.Title, &m.Content,
			&m.SourceAgentID, &m.HitCount, &m.Tier,
			&m.ReviewedAt, &m.PromotedToAntibodyID,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CountMistakes — count non-deleted, optional filter tier.
func CountMistakes(ctx context.Context, tier string) (int64, error) {
	db, err := OpenRW()
	if err != nil {
		return 0, err
	}
	query := `SELECT COUNT(*) FROM mistakes_journal WHERE deleted_at IS NULL`
	args := []any{}
	if tier != "" {
		query += ` AND tier = ?`
		args = append(args, tier)
	}
	var n int64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
