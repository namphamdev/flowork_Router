// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 1 (Ingestion pipeline) DONE + adversarial-audit passed.
//   API stable: Req struct, Submit(), SubmitBatch(), Summarize(). End-to-end
//   verified (single, dedupe, batch dengan stats agregat akurat). Future
//   extension (embedding gen Section 5, federation source Section 17, async
//   worker pool) → TAMBAH function/file baru di package ini, JANGAN ubah
//   Submit() signature atau Req struct field yang ada.
//
// Package ingest — pipeline orchestrator untuk grow brain. Input → sanitize
// → score → dedupe (via brain.AddDrawerFull content_hash) → write drawer +
// FTS. Caller pakai via Submit() (single) atau SubmitBatch() (banyak).
//
// Source: flowork_Router/roadmap.md Section 1.
//
// Standar yang dipakai:
//   - source_type taxonomy: 'manual' | 'chat' | 'doc' | 'federation' | 'compounding'
//   - mem_type taxonomy:    'project' | 'compounding' | 'antibody' | 'fact' | 'skill'
//   - importance scale:     0.0–10.0 (default 3.0 untuk anonymous submit)
//
// Anti over-engineer: package ini cuma orchestrator. Embedding + FTS sync
// di brain.AddDrawerFull. Score heuristic kecil — re-score job di section 2
// kerjain refine.
package ingest

import (
	"context"
	"fmt"

	"github.com/flowork-os/flowork_Router/internal/brain"
)

// Req — payload satu drawer baru. Field optional kecuali Content.
//
//	Content     wajib, > 0 char setelah sanitize
//	Wing/Room   organisasi knowledge palace (default wing="compounding")
//	SourceType  taxonomy asal: 'manual' (API), 'chat' (compounding), 'doc' (import), 'federation'
//	SourceFile  identifier asal (path file, chat ID, dst.)
//	MemType     taxonomy purpose: 'project', 'compounding', 'antibody', 'fact'
//	Importance  0-10. Kalau 0 → di-score otomatis.
//	ChunkIndex  > 0 untuk chunk N dari doc panjang (dipakai SubmitBatch dari docs)
type Req struct {
	Content    string  `json:"content"`
	Wing       string  `json:"wing,omitempty"`
	Room       string  `json:"room,omitempty"`
	SourceType string  `json:"source_type,omitempty"`
	SourceFile string  `json:"source_file,omitempty"`
	MemType    string  `json:"mem_type,omitempty"`
	Importance float64 `json:"importance,omitempty"`
	ChunkIndex int     `json:"chunk_index,omitempty"`
}

// Result — outcome satu submit. DrawerID di-return walaupun deduped (caller
// bisa link ke drawer existing untuk audit).
type Result struct {
	DrawerID string `json:"drawer_id"`
	Added    bool   `json:"added"`           // false → content_hash sudah ada (dedupe hit)
	Note     string `json:"note,omitempty"`  // alasan skip atau warning
	Error    string `json:"error,omitempty"` // populated kalau gagal — caller cek len(Error) > 0
}

// Submit — orchestrate satu drawer ingestion: sanitize → score (kalau perlu)
// → write via brain.AddDrawerFull.
//
// Return Result dengan DrawerID + Added flag. Error embedded di Result.Error
// supaya batch loop ngga short-circuit di satu failure.
func Submit(ctx context.Context, req Req) Result {
	content := Sanitize(req.Content)
	if content == "" {
		return Result{Error: "content empty after sanitize"}
	}
	if minContentChars > 0 && len(content) < minContentChars {
		return Result{Note: fmt.Sprintf("skipped: content < %d chars", minContentChars)}
	}
	importance := req.Importance
	if importance <= 0 {
		importance = Score(content, req.SourceType)
	}

	id, added, err := brain.AddDrawerFull(ctx, brain.AddDrawerOpts{
		Content:    content,
		Wing:       req.Wing,
		Room:       req.Room,
		SourceType: req.SourceType,
		SourceFile: req.SourceFile,
		MemType:    req.MemType,
		Importance: importance,
		ChunkIndex: req.ChunkIndex,
	})
	if err != nil {
		return Result{Error: err.Error()}
	}
	return Result{DrawerID: id, Added: added}
}

// SubmitBatch — N items, return slice Result paralel. Tidak short-circuit
// pada error satu item. Caller agregat count via Result.Added bool.
//
// MAX items: hard cap di handler (default 1000) supaya satu request ngga
// monopoli writer.
func SubmitBatch(ctx context.Context, items []Req) []Result {
	out := make([]Result, 0, len(items))
	for _, it := range items {
		out = append(out, Submit(ctx, it))
	}
	return out
}

// BatchStats — agregat result batch untuk reporting di response.
type BatchStats struct {
	Total   int `json:"total"`
	Added   int `json:"added"`
	Deduped int `json:"deduped"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// Summarize — hitung BatchStats dari slice Result.
func Summarize(results []Result) BatchStats {
	var s BatchStats
	s.Total = len(results)
	for _, r := range results {
		switch {
		case r.Error != "":
			s.Failed++
		case r.Note != "":
			s.Skipped++
		case r.Added:
			s.Added++
		default:
			s.Deduped++
		}
	}
	return s
}

// minContentChars — quality gate: drawer < N char di-skip (terlalu pendek
// untuk knowledge). 20 char align sama compounding stub di
// handlers_brain_views.go::brainIngestRunHandler.
const minContentChars = 20
