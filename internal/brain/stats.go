// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Brain drawer/embedding/skills storage.

package brain

import (
	"context"
	"os"
)

// WingCount — drawer count for one wing.
type WingCount struct {
	Wing  string `json:"wing"`
	Count int64  `json:"count"`
}

// Stats — a snapshot of the brain DB for the dashboard.
type Stats struct {
	Available bool        `json:"available"`
	Path      string      `json:"path"`
	SizeBytes int64       `json:"sizeBytes"`
	Drawers   int64       `json:"drawers"`
	Wings     []WingCount `json:"wings"`
	Skills    int         `json:"skills"` // embedded skill library size
}

// GetStats reports availability + lightweight content stats. Counts are
// best-effort: a query error leaves the field zero rather than failing.
func GetStats(ctx context.Context) Stats {
	st := Stats{Path: DBPath(), Skills: len(Skills())}
	if !Available() {
		return st
	}
	st.Available = true
	if info, err := os.Stat(st.Path); err == nil {
		st.SizeBytes = info.Size()
	}
	db, err := Open()
	if err != nil {
		return st
	}
	// Count ALL drawers (not just deleted_at IS NULL): the FTS index keeps every
	// chunk regardless of the drawers tombstone, so RAG retrieval actually searches
	// the full corpus. Report what the brain can really retrieve.
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM drawers`).Scan(&st.Drawers)
	rows, err := db.QueryContext(ctx, `SELECT wing, COUNT(*) c FROM drawers
		GROUP BY wing ORDER BY c DESC LIMIT 12`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var w WingCount
			if err := rows.Scan(&w.Wing, &w.Count); err == nil {
				st.Wings = append(st.Wings, w)
			}
		}
	}
	return st
}
