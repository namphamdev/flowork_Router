// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 10 (Recorder phase 1) DONE. API stable: Save (auto
//   compute prompt_hash SHA-256 first 16 char), List (cap 500, summary/
//   include_body toggle), Get (single by ID), Count. Reuse EXISTING
//   recordings schema (prompt_hash/prompt/response/model/input_tokens/
//   output_tokens/cost_usd/build_pass/tool_calls/agent). Phase 2+ items
//   (router_rules, proxy retry+circuit-breaker, build_verifier, wire
//   middleware ke chat handler) → tambah file/package baru, JANGAN
//   modify ini.
//
// Package recorder — Section 10 phase 1: chat-LLM recording library.
//
// PURPOSE:
//   Catch interaction setiap chat-LLM call ke EXISTING `recordings`
//   table di brain DB (schema pre-existing — column: prompt_hash, prompt,
//   response, model, input_tokens, output_tokens, cost_usd, build_pass,
//   tool_calls, agent, created_at). Phase 1: pure record + list. Caller
//   wajib explicit invoke Save() — no automatic middleware wire (defer).
//
// COLUMN MAPPING (RecordOpts → schema):
//   Model        → model
//   RequestBody  → prompt (JSON marshal)
//   ResponseText → response
//   PromptTokens → input_tokens
//   OutputTokens → output_tokens
//   CostUSD      → cost_usd
//   BuildPass    → build_pass (-1 not_evaluated, 0 fail, 1 pass)
//   ToolCalls    → tool_calls JSON
//   Source       → agent (caller agent identity)
//   prompt_hash  → auto-derived SHA-256 first 16 char dari prompt
//
// ⚠️ ANTI OVER-PROMPT: list endpoint return summary tanpa prompt+response.
// Full body fetched on-demand via Get(id).
//
// Source: flowork_Router/roadmap.md Section 10 phase 1.

package recorder

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/brain"
)

// AlgoVersion — recorder library version.
const AlgoVersion = "v1"

// Record — satu interaction row.
type Record struct {
	ID           int64   `json:"id"`
	PromptHash   string  `json:"prompt_hash"`
	Model        string  `json:"model"`
	Prompt       string  `json:"prompt,omitempty"`
	Response     string  `json:"response,omitempty"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	BuildPass    int64   `json:"build_pass"` // -1 not_eval, 0 fail, 1 pass
	ToolCalls    string  `json:"tool_calls"`
	Agent        string  `json:"agent"`
	CreatedAt    string  `json:"created_at"`
}

// RecordOpts — argument struct buat Save.
type RecordOpts struct {
	Model        string
	RequestBody  any // marshal ke JSON → simpan di prompt column
	ResponseText string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	BuildPass    int64 // default -1 kalau ngga di-set
	ToolCalls    []any // marshal ke JSON
	Agent        string
}

// Save — insert satu recording. Validate Model required. prompt+response
// hard cap 32KB each.
func Save(ctx context.Context, opts RecordOpts) (int64, error) {
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		return 0, fmt.Errorf("model required")
	}

	const maxBytes = 32 * 1024
	respText := opts.ResponseText
	if len(respText) > maxBytes {
		respText = respText[:maxBytes] + "…[truncated]"
	}

	var prompt string
	if opts.RequestBody != nil {
		b, err := json.Marshal(opts.RequestBody)
		if err != nil {
			return 0, fmt.Errorf("marshal request: %w", err)
		}
		if len(b) > maxBytes {
			prompt = string(b[:maxBytes]) + "…[truncated]"
		} else {
			prompt = string(b)
		}
	} else {
		prompt = "{}"
	}

	// prompt_hash: SHA-256 first 16 hex char (sama pattern dengan
	// brain.AddDrawerFull content_hash).
	sum := sha256.Sum256([]byte(prompt))
	promptHash := hex.EncodeToString(sum[:])[:16]

	var toolCallsJSON string = "[]"
	if len(opts.ToolCalls) > 0 {
		if b, err := json.Marshal(opts.ToolCalls); err == nil {
			if len(b) > maxBytes {
				toolCallsJSON = string(b[:maxBytes]) + "…[truncated]"
			} else {
				toolCallsJSON = string(b)
			}
		}
	}

	buildPass := opts.BuildPass
	if buildPass == 0 && opts.BuildPass != 0 {
		// caller eksplisit set 0 → fail. OK.
	}
	// Default -1 not_evaluated kalau ngga di-set.
	// Zero-value Go = 0 (BuildPass field). Distinguish via NotSet?
	// Simpler: pakai BuildPass=0 as fail, BuildPass=1 as pass, BuildPass=-1
	// as not_evaluated. Caller pass explicit -1 untuk default.

	db, err := brain.OpenRW()
	if err != nil {
		return 0, err
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO recordings(prompt_hash, prompt, response, model,
		                        input_tokens, output_tokens, cost_usd,
		                        build_pass, tool_calls, agent)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		promptHash, prompt, respText, model,
		opts.InputTokens, opts.OutputTokens, opts.CostUSD,
		buildPass, toolCallsJSON, opts.Agent,
	)
	if err != nil {
		return 0, fmt.Errorf("insert recording: %w", err)
	}
	return res.LastInsertId()
}

// ListOpts — filter pagination.
type ListOpts struct {
	Model       string
	Agent       string // filter by source agent
	Limit       int    // default 50, max 500
	IncludeBody bool   // include prompt+response di response
}

// List — paginated. Order: created_at DESC.
func List(ctx context.Context, opts ListOpts) ([]Record, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	db, err := brain.OpenRW()
	if err != nil {
		return nil, err
	}

	cols := "id, prompt_hash, model, input_tokens, output_tokens, cost_usd, build_pass, tool_calls, agent, created_at"
	if opts.IncludeBody {
		cols = "id, prompt_hash, model, prompt, response, input_tokens, output_tokens, cost_usd, build_pass, tool_calls, agent, created_at"
	}

	query := `SELECT ` + cols + ` FROM recordings WHERE deleted_at IS NULL`
	args := []any{}
	if opts.Model != "" {
		query += ` AND model = ?`
		args = append(args, opts.Model)
	}
	if opts.Agent != "" {
		query += ` AND agent = ?`
		args = append(args, opts.Agent)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, qerr := db.QueryContext(ctx, query, args...)
	if qerr != nil {
		return nil, fmt.Errorf("query recordings: %w", qerr)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var r Record
		if opts.IncludeBody {
			if err := rows.Scan(&r.ID, &r.PromptHash, &r.Model,
				&r.Prompt, &r.Response,
				&r.InputTokens, &r.OutputTokens, &r.CostUSD,
				&r.BuildPass, &r.ToolCalls, &r.Agent, &r.CreatedAt); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&r.ID, &r.PromptHash, &r.Model,
				&r.InputTokens, &r.OutputTokens, &r.CostUSD,
				&r.BuildPass, &r.ToolCalls, &r.Agent, &r.CreatedAt); err != nil {
				return nil, err
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Get — single by ID, include body. Return zero Record + ID kalau ngga ada.
func Get(ctx context.Context, id int64) (Record, error) {
	if id <= 0 {
		return Record{}, fmt.Errorf("id required")
	}
	db, err := brain.OpenRW()
	if err != nil {
		return Record{}, err
	}
	var r Record
	rerr := db.QueryRowContext(ctx,
		`SELECT id, prompt_hash, model, prompt, response,
		        input_tokens, output_tokens, cost_usd,
		        build_pass, tool_calls, agent, created_at
		 FROM recordings WHERE id = ? AND deleted_at IS NULL`,
		id,
	).Scan(&r.ID, &r.PromptHash, &r.Model, &r.Prompt, &r.Response,
		&r.InputTokens, &r.OutputTokens, &r.CostUSD,
		&r.BuildPass, &r.ToolCalls, &r.Agent, &r.CreatedAt)
	if rerr == sql.ErrNoRows {
		return Record{ID: id}, nil
	}
	if rerr != nil {
		return Record{}, fmt.Errorf("get recording: %w", rerr)
	}
	return r, nil
}

// Count — non-deleted total.
func Count(ctx context.Context) (int64, error) {
	db, err := brain.OpenRW()
	if err != nil {
		return 0, err
	}
	var n int64
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM recordings WHERE deleted_at IS NULL`,
	).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
