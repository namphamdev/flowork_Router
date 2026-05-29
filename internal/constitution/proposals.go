// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Section 12 (Discipline constitution phase 1) DONE — leverage
//   EXISTING `constitution` table schema dengan pending_quorum_review
//   column + auto-trigger `trg_constitution_log_update` (history).
//   API stable: Propose (INSERT pending=1, UNIQUE source_file+section),
//   ListPending (cap 500, summary/include_content toggle), Vote
//   (approve|reject, idempotent via no-op kalau already settled),
//   CountPending. Single-owner phase 1: VoterID free-form via header.
//   Phase 2/3 (multi-signer quorum auth, brain reset, content edit
//   propose workflow) → tambah function/file baru, JANGAN modify ini.
//
// Package constitution — Section 12 phase 1: propose + vote workflow.
//
// PURPOSE:
//   Constitution edit ada governance: propose dulu (pending_quorum_review=1),
//   approval/reject via vote. History preserved via existing TRIGGER
//   `trg_constitution_log_update` (auto-archive ke constitution_history).
//
// SCHEMA REUSE:
//   Existing table `constitution` punya `pending_quorum_review INTEGER DEFAULT 0`.
//   - Proposal new = INSERT pending_quorum_review=1
//   - Proposal pending = pending_quorum_review=1 (kepilih review)
//   - Approved = pending_quorum_review=0 (active rule, trigger sync history)
//   - Rejected = soft-delete (deleted_at + deleted_by='vote-rejected')
//
// SECURITY:
//   - Phase 1 single-owner: caller dispute via header `X-Voter-ID` (free-form
//     identitas). Phase 2 add real auth — multi-signer quorum.
//   - Vote action whitelist: approve | reject.
//
// ⚠️ Anti over-prompt: list pending endpoint summary-only (no content body).
// Full body via dedicated GET endpoint kalau caller butuh review detail.
//
// Source: flowork_Router/roadmap.md Section 12 phase 1.

package constitution

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/flowork-os/flowork_Router/internal/brain"
)

// AlgoVersion — proposal workflow version.
const AlgoVersion = "v1"

// Proposal — single pending rule.
type Proposal struct {
	ID         int64   `json:"id"`
	SourceFile string  `json:"source_file"`
	Section    string  `json:"section"`
	Content    string  `json:"content,omitempty"`
	Amplitude  float64 `json:"amplitude"`
	CreatedAt  string  `json:"created_at,omitempty"`
}

// ProposeOpts — argument struct buat Propose.
type ProposeOpts struct {
	SourceFile string
	Section    string
	Content    string
	Amplitude  float64
	ContextOrigin string // optional — alasan / reference (audit trail)
	Signer        string // proposer identity (free-form)
}

// Propose — INSERT row baru dengan pending_quorum_review=1. UNIQUE
// constraint (source_file, section): caller tidak boleh propose duplikat.
// Return (id, error).
func Propose(ctx context.Context, opts ProposeOpts) (int64, error) {
	sourceFile := strings.TrimSpace(opts.SourceFile)
	section := strings.TrimSpace(opts.Section)
	content := strings.TrimSpace(opts.Content)
	if sourceFile == "" || section == "" || content == "" {
		return 0, fmt.Errorf("source_file + section + content required")
	}
	const (
		maxText  = 16 * 1024 // 16KB content cap
		maxField = 256
	)
	if len(content) > maxText {
		content = content[:maxText] + "…[truncated]"
	}
	if len(sourceFile) > maxField {
		sourceFile = sourceFile[:maxField]
	}
	if len(section) > maxField {
		section = section[:maxField]
	}

	amp := opts.Amplitude
	if amp <= 0 {
		amp = 1.0 // default neutral
	}

	signer := strings.TrimSpace(opts.Signer)
	if signer == "" {
		signer = "anonymous"
	}

	db, err := brain.OpenRW()
	if err != nil {
		return 0, err
	}

	res, ierr := db.ExecContext(ctx,
		`INSERT INTO constitution(source_file, section, content, amplitude,
		                          pending_quorum_review, context_origin, signer, origin_node)
		 VALUES(?, ?, ?, ?, 1, ?, ?, 'local')`,
		sourceFile, section, content, amp, opts.ContextOrigin, signer,
	)
	if ierr != nil {
		return 0, fmt.Errorf("insert proposal: %w", ierr)
	}
	return res.LastInsertId()
}

// ListPending — list rows pending_quorum_review=1. Order: id ASC (oldest first).
// Default limit 50, max 500. Body cuma di-include kalau includeContent=true.
func ListPending(ctx context.Context, limit int, includeContent bool) ([]Proposal, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	db, err := brain.OpenRW()
	if err != nil {
		return nil, err
	}

	cols := "id, source_file, section, amplitude"
	if includeContent {
		cols = "id, source_file, section, content, amplitude"
	}

	rows, qerr := db.QueryContext(ctx,
		`SELECT `+cols+`
		 FROM constitution
		 WHERE pending_quorum_review = 1 AND deleted_at IS NULL
		 ORDER BY id ASC LIMIT ?`,
		limit,
	)
	if qerr != nil {
		return nil, fmt.Errorf("query pending: %w", qerr)
	}
	defer rows.Close()

	var out []Proposal
	for rows.Next() {
		var p Proposal
		if includeContent {
			if err := rows.Scan(&p.ID, &p.SourceFile, &p.Section, &p.Content, &p.Amplitude); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&p.ID, &p.SourceFile, &p.Section, &p.Amplitude); err != nil {
				return nil, err
			}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// VoteOpts — argument struct.
type VoteOpts struct {
	ProposalID int64
	Action     string // "approve" | "reject"
	VoterID    string // free-form identitas
}

// VoteResult — outcome vote.
type VoteResult struct {
	ProposalID int64  `json:"proposal_id"`
	Action     string `json:"action"`
	Status     string `json:"status"` // "approved" | "rejected" | "no-op"
	VoterID    string `json:"voter_id"`
}

// Vote — apply action ke pending proposal.
//   - approve → SET pending_quorum_review=0 (active rule, trigger logs history)
//   - reject  → soft-delete (deleted_at + deleted_by='vote-rejected:<voter>')
//
// Return VoteResult. Idempotent: kalau row sudah promoted/rejected, status="no-op".
func Vote(ctx context.Context, opts VoteOpts) (VoteResult, error) {
	r := VoteResult{ProposalID: opts.ProposalID, Action: opts.Action, VoterID: opts.VoterID}
	if opts.ProposalID <= 0 {
		return r, fmt.Errorf("proposal_id required")
	}
	action := strings.TrimSpace(opts.Action)
	if action != "approve" && action != "reject" {
		return r, fmt.Errorf("action must be 'approve' or 'reject'")
	}
	voter := strings.TrimSpace(opts.VoterID)
	if voter == "" {
		voter = "anonymous"
	}

	db, err := brain.OpenRW()
	if err != nil {
		return r, err
	}

	// Lookup current state untuk idempotency check.
	var pending int
	var deletedAt sql.NullString
	qerr := db.QueryRowContext(ctx,
		`SELECT pending_quorum_review, deleted_at FROM constitution WHERE id = ?`,
		opts.ProposalID,
	).Scan(&pending, &deletedAt)
	if qerr == sql.ErrNoRows {
		return r, fmt.Errorf("proposal not found")
	}
	if qerr != nil {
		return r, fmt.Errorf("lookup: %w", qerr)
	}
	if deletedAt.Valid {
		r.Status = "no-op"
		return r, nil
	}
	if pending == 0 {
		// Already promoted (active). No-op.
		r.Status = "no-op"
		return r, nil
	}

	switch action {
	case "approve":
		_, uerr := db.ExecContext(ctx,
			`UPDATE constitution SET pending_quorum_review = 0 WHERE id = ?`,
			opts.ProposalID,
		)
		if uerr != nil {
			return r, fmt.Errorf("approve: %w", uerr)
		}
		r.Status = "approved"
	case "reject":
		ts := time.Now().UTC().Format(time.RFC3339)
		deletedBy := "vote-rejected:" + voter
		_, uerr := db.ExecContext(ctx,
			`UPDATE constitution SET deleted_at = ?, deleted_by = ? WHERE id = ?`,
			ts, deletedBy, opts.ProposalID,
		)
		if uerr != nil {
			return r, fmt.Errorf("reject: %w", uerr)
		}
		r.Status = "rejected"
	}
	return r, nil
}

// CountPending — pending_quorum_review=1 + not soft-deleted.
func CountPending(ctx context.Context) (int64, error) {
	db, err := brain.OpenRW()
	if err != nil {
		return 0, err
	}
	var n int64
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM constitution WHERE pending_quorum_review = 1 AND deleted_at IS NULL`,
	).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
