// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 2 (Importance scorer re-score) phase 1 DONE +
//   adversarial-audit passed (C1 preserve manual-curation via threshold
//   1.5 + ForceOverride bypass + atomic tx wrap). API stable:
//   RescoreOpts, RescoreReport (5 stat fields), RescoreBatch.
//   Verified: default preserve mode keeps manual=10, force_override=true
//   overwrites to heuristic 5.5. Future cron + retrieval-frequency
//   tracking → tambah file baru / kolom drawer schema, JANGAN modify.
//
// rescore.go — Section 2 roadmap: Importance scorer re-score worker.
//
// PURPOSE:
//   Re-compute `drawers.importance` untuk live (non-deleted, non-quarantined)
//   drawer. Pakai scorer function caller (typically `ingest.Score`) supaya
//   ngga circular import brain ↔ ingest. Batch update — return per-drawer
//   delta untuk audit.
//
// Phase 1 scope:
//   - Filter optional: wing (only re-score drawer di wing tertentu)
//   - Limit: bounded batch size supaya 1 request ngga monopoli writer
//   - Update via UPDATE drawers SET importance = ? WHERE id = ? (atomic
//     transaction wrapping seluruh batch)
//
// Defer:
//   - Cron weekly trigger
//   - Retrieval-frequency factor (butuh kolom retrieval_count atau hits
//     table — schema extend nanti)
//
// ⚠️ Anti over-prompt: importance dipakai retrieval rank — bagus. Tapi
// JANGAN auto-inject "this drawer has importance X" ke system prompt
// (lihat standar section 11).

package brain

import (
	"context"
	"fmt"
)

// RescoreOpts — parameter batch re-score.
type RescoreOpts struct {
	// Wing kosong = scan semua wing. Filter wing tertentu = limit scope.
	Wing string
	// Limit max drawer di-process per call. Default 1000, max 10000.
	Limit int
	// ForceOverride: kalau true, rescore overwrite SEMUA drawer termasuk
	// yang importance-nya dekat heuristic baru. Default false: preserve
	// drawer dengan importance yang jauh dari heuristic (asumsi: caller
	// manual-set explicit value via /api/brain/ingest/submit dengan
	// Importance > 0 — itu curation, jangan auto-overwrite).
	// Threshold preserve: abs(current - heuristic) > preserveThreshold.
	ForceOverride bool
}

// preserveThreshold — drawer dengan abs(current importance - heuristic)
// di atas threshold ini di-anggap manual curation. Rescore default skip.
// 1.5 cover variasi heuristic kecil (signal word tambah/kurang, length
// boost edge) tapi flag caller explicit set (mis. 7.0 vs heuristic 5.0).
const preserveThreshold = 1.5

// RescoreReport — agregat hasil batch re-score. PerDrawer optional log
// untuk admin audit (di-cap supaya response ngga balloon).
type RescoreReport struct {
	Scanned     int            `json:"scanned"`
	Updated     int            `json:"updated"`
	Unchanged   int            `json:"unchanged"`
	Preserved   int            `json:"preserved"` // manual-curation, skipped unless ForceOverride
	Errors      []string       `json:"errors,omitempty"`
	SampleDelta []RescoreDelta `json:"sample_delta,omitempty"` // first 20 untuk preview
}

// RescoreDelta — single drawer importance change.
type RescoreDelta struct {
	DrawerID string  `json:"drawer_id"`
	Before   float64 `json:"before"`
	After    float64 `json:"after"`
}

// scorerFn — caller inject scorer function (typically ingest.Score) supaya
// brain pkg ngga circular import ke ingest.
type scorerFn func(content, sourceType string) float64

// RescoreBatch — iterate live drawers (deleted_at IS NULL, quarantined=0)
// dengan filter optional wing + limit, recompute importance via scorer fn,
// UPDATE kalau berbeda. Transaction-wrapped untuk atomicity.
//
// Caller bertanggung jawab inject scorer (typically `ingest.Score`) supaya
// ngga circular import. Sentinel: kalau scorer nil → return error.
func RescoreBatch(ctx context.Context, opts RescoreOpts, scorer scorerFn) (RescoreReport, error) {
	if scorer == nil {
		return RescoreReport{}, fmt.Errorf("scorer function required")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 1000
	}
	if limit > 10000 {
		limit = 10000
	}

	db, err := OpenRW()
	if err != nil {
		return RescoreReport{}, err
	}

	// SELECT live drawer dengan filter wing optional. Ambil id + content +
	// source_type + current importance — semua field yang dibutuhkan
	// scorer + comparison.
	query := `SELECT id, content, source_type, importance
	          FROM drawers
	          WHERE deleted_at IS NULL AND quarantined = 0`
	args := []any{}
	if opts.Wing != "" {
		query += ` AND wing = ?`
		args = append(args, opts.Wing)
	}
	query += ` ORDER BY filed_at DESC LIMIT ?`
	args = append(args, limit)

	rows, qerr := db.QueryContext(ctx, query, args...)
	if qerr != nil {
		return RescoreReport{}, fmt.Errorf("query drawers: %w", qerr)
	}

	// Collect to memory dulu — release rows reader sebelum begin tx (SQLite
	// ngga suka rowsCursor terbuka saat write).
	type drawerRow struct {
		id         string
		content    string
		sourceType string
		importance float64
	}
	var drawers []drawerRow
	for rows.Next() {
		var d drawerRow
		if err := rows.Scan(&d.id, &d.content, &d.sourceType, &d.importance); err != nil {
			rows.Close()
			return RescoreReport{}, fmt.Errorf("scan drawer: %w", err)
		}
		drawers = append(drawers, d)
	}
	rows.Close()
	if rerr := rows.Err(); rerr != nil {
		return RescoreReport{}, fmt.Errorf("iterate drawers: %w", rerr)
	}

	rep := RescoreReport{Scanned: len(drawers)}

	// Pre-compute deltas + decide which need update. 3 outcome per drawer:
	//   - diff < 0.01 → Unchanged (float noise, skip)
	//   - diff > preserveThreshold && !ForceOverride → Preserved (manual
	//     curation likely — skip unless forced)
	//   - else → pending update
	type pendingUpdate struct {
		id       string
		newScore float64
		oldScore float64
	}
	var pending []pendingUpdate
	for _, d := range drawers {
		newScore := scorer(d.content, d.sourceType)
		delta := absDelta(newScore, d.importance)
		if delta < 0.01 {
			rep.Unchanged++
			continue
		}
		if !opts.ForceOverride && delta > preserveThreshold {
			// Likely caller manual-set via Importance > 0 di ingest.Req.
			// Audit Section 2 finding — preserve curation.
			rep.Preserved++
			continue
		}
		pending = append(pending, pendingUpdate{
			id:       d.id,
			newScore: newScore,
			oldScore: d.importance,
		})
	}

	if len(pending) == 0 {
		return rep, nil
	}

	// Atomic batch update — kalau crash di tengah, rollback semua. Mencegah
	// state inkonsisten (sebagian drawer di-rescore, sebagian belum).
	tx, terr := db.BeginTx(ctx, nil)
	if terr != nil {
		return rep, fmt.Errorf("begin tx: %w", terr)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, perr := tx.PrepareContext(ctx, `UPDATE drawers SET importance = ? WHERE id = ?`)
	if perr != nil {
		return rep, fmt.Errorf("prepare update: %w", perr)
	}
	defer stmt.Close()

	for _, p := range pending {
		if _, uerr := stmt.ExecContext(ctx, p.newScore, p.id); uerr != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("drawer %s: %s", p.id, uerr.Error()))
			continue
		}
		rep.Updated++
		if len(rep.SampleDelta) < 20 {
			rep.SampleDelta = append(rep.SampleDelta, RescoreDelta{
				DrawerID: p.id,
				Before:   p.oldScore,
				After:    p.newScore,
			})
		}
	}

	if cerr := tx.Commit(); cerr != nil {
		return rep, fmt.Errorf("commit tx: %w", cerr)
	}
	tx = nil
	return rep, nil
}

// absDelta — abs(a-b) tanpa import math (anti dep).
func absDelta(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
