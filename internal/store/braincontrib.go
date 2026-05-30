// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Brain drawer/embeddings/skills.

package store

import (
	"database/sql"
	"time"
)

// BrainContribution — one recorded brain interaction, queued for ingestion back
// into the master knowledge brain (the compounding loop). flow_router only
// QUEUES contributions here (its own writable DB); ingesting them into the
// large read-only brain DB is the single-writer's job (e.g. flowork's ingestor
// or an export job), which keeps the brain DB safe for concurrent access.
type BrainContribution struct {
	ID       int64  `json:"id"`
	TS       string `json:"ts"`
	Agent    string `json:"agent"` // calling API key name, or "anonymous"
	Model    string `json:"model"`
	Mode     string `json:"mode"`
	Query    string `json:"query"`
	Sources  string `json:"sources"` // JSON array of retrieved {drawerId,wing,score}
	Answer   string `json:"answer"`
	Ingested bool   `json:"ingested"`
}

// AddBrainContribution appends one interaction (append-only; never updated
// except for the ingested flag).
func AddBrainContribution(d *sql.DB, c BrainContribution) error {
	_, err := d.Exec(`INSERT INTO brainContributions (ts, agent, model, mode, query, sources, answer, ingested)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)`,
		time.Now().UTC().Format(time.RFC3339), c.Agent, c.Model, c.Mode, c.Query, c.Sources, c.Answer)
	return err
}

// ListBrainContributions returns contributions newest-first. pendingOnly limits
// to not-yet-ingested rows. limit <=0 defaults to 200.
func ListBrainContributions(d *sql.DB, pendingOnly bool, limit int) ([]BrainContribution, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT id, ts, agent, model, mode, query, sources, answer, ingested FROM brainContributions`
	if pendingOnly {
		q += ` WHERE ingested = 0`
	}
	q += ` ORDER BY id DESC LIMIT ?`
	rows, err := d.Query(q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BrainContribution
	for rows.Next() {
		var c BrainContribution
		var ing int
		if err := rows.Scan(&c.ID, &c.TS, &c.Agent, &c.Model, &c.Mode, &c.Query, &c.Sources, &c.Answer, &ing); err != nil {
			continue
		}
		c.Ingested = ing == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountBrainContributions returns total and pending (not-yet-ingested) counts.
func CountBrainContributions(d *sql.DB) (total, pending int) {
	_ = d.QueryRow(`SELECT COUNT(*) FROM brainContributions`).Scan(&total)
	_ = d.QueryRow(`SELECT COUNT(*) FROM brainContributions WHERE ingested = 0`).Scan(&pending)
	return total, pending
}

// MarkContributionsIngested flips the ingested flag for all rows up to and
// including maxID (the contract an ingestor uses after consuming a batch).
func MarkContributionsIngested(d *sql.DB, maxID int64) (int64, error) {
	res, err := d.Exec(`UPDATE brainContributions SET ingested = 1 WHERE id <= ? AND ingested = 0`, maxID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
