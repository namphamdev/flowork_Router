// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Store SQLite layer.

// Request Log Store (Console Log + Usage).

package store

import (
	"database/sql"
	"fmt"
	"time"
)

// LogEntry — single request log entry.
type LogEntry struct {
	ID               int64     `json:"id"`
	Ts               time.Time `json:"ts"`
	ProviderID       string    `json:"providerId,omitempty"`
	ProviderName     string    `json:"providerName,omitempty"`
	APIKeyID         string    `json:"apiKeyId,omitempty"` // inbound key attribution ("" = anonymous)
	Model            string    `json:"model"`
	ClientIP         string    `json:"clientIp,omitempty"`
	StatusCode       int       `json:"statusCode"`
	Error            string    `json:"error,omitempty"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	CostUsd          float64   `json:"costUsd"`
	LatencyMs        int64     `json:"latencyMs"`
}

// LogRequest — append-only insert. Best-effort, log error tapi ngga gagal caller.
func LogRequest(d *sql.DB, e *LogEntry) error {
	now := time.Now().UTC()
	if e.Ts.IsZero() {
		e.Ts = now
	}
	tsStr := e.Ts.Format(time.RFC3339)
	dayStr := e.Ts.Format("2006-01-02")

	// Insert ke usageHistory (lightweight)
	_, err := d.Exec(`INSERT INTO usageHistory (ts, provider, model, apiKeyId, promptTokens, completionTokens, costUsd, latencyMs, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tsStr, e.ProviderID, e.Model, e.APIKeyID,
		e.PromptTokens, e.CompletionTokens, e.CostUsd, e.LatencyMs, statusText(e.StatusCode, e.Error))
	if err != nil {
		return fmt.Errorf("usageHistory insert: %w", err)
	}

	// Upsert usageDaily aggregate
	_, _ = d.Exec(`INSERT INTO usageDaily (day, provider, model, apiKeyId, requestCount, promptTokens, completionTokens, costUsd)
		VALUES (?, ?, ?, ?, 1, ?, ?, ?)
		ON CONFLICT(day, provider, model, apiKeyId) DO UPDATE SET
			requestCount = requestCount + 1,
			promptTokens = promptTokens + excluded.promptTokens,
			completionTokens = completionTokens + excluded.completionTokens,
			costUsd = costUsd + excluded.costUsd`,
		dayStr, e.ProviderID, e.Model, e.APIKeyID,
		e.PromptTokens, e.CompletionTokens, e.CostUsd)

	// Auto-prune kalau > 10000 rows
	_, _ = d.Exec(`DELETE FROM usageHistory WHERE id IN (
		SELECT id FROM usageHistory ORDER BY id ASC LIMIT (
			SELECT MAX(0, COUNT(*) - 10000) FROM usageHistory
		)
	)`)

	return nil
}

func statusText(code int, err string) string {
	if err != "" {
		return "error"
	}
	if code >= 200 && code < 300 {
		return "ok"
	}
	if code >= 400 && code < 500 {
		return "client_error"
	}
	if code >= 500 {
		return "server_error"
	}
	return "unknown"
}

// ListRecent — return last N entries (default 100, max 1000).
// Optional filter: provider, model, status.
func ListRecent(d *sql.DB, limit int, providerFilter, statusFilter string) ([]LogEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	q := `SELECT id, ts, provider, model, promptTokens, completionTokens, costUsd, latencyMs, status
		FROM usageHistory WHERE 1=1`
	args := []any{}
	if providerFilter != "" {
		q += ` AND provider = ?`
		args = append(args, providerFilter)
	}
	if statusFilter != "" {
		// "error" means "anything that is not a clean ok" — so it also catches
		// client_error / server_error / unknown, not just rows tagged "error".
		if statusFilter == "error" {
			q += ` AND status != 'ok'`
		} else {
			q += ` AND status = ?`
			args = append(args, statusFilter)
		}
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		var status, tsStr string
		if err := rows.Scan(&e.ID, &tsStr, &e.ProviderID, &e.Model,
			&e.PromptTokens, &e.CompletionTokens, &e.CostUsd, &e.LatencyMs, &status); err != nil {
			return nil, err
		}
		e.Ts, _ = time.Parse(time.RFC3339, tsStr)
		e.TotalTokens = e.PromptTokens + e.CompletionTokens
		switch status {
		case "error":
			e.Error = status
			e.StatusCode = 500
		case "client_error":
			e.StatusCode = 400
		case "server_error":
			e.StatusCode = 502
		default:
			e.StatusCode = 200
		}
		out = append(out, e)
	}

	// Resolve provider IDs → human-friendly names. The log only stores the
	// provider ID; without this the UI can show nothing but a cryptic UUID
	// fragment (ProviderName was previously never populated). Deleted providers
	// keep their ID (the UI falls back to a short prefix).
	if len(out) > 0 {
		names := map[string]string{}
		if prows, perr := d.Query(`SELECT id, provider, name FROM providerConnections`); perr == nil {
			func() {
				defer prows.Close()
				for prows.Next() {
					var id, ptype, name string
					if prows.Scan(&id, &ptype, &name) == nil {
						if name == "" {
							name = ptype
						}
						names[id] = name
					}
				}
			}()
		}
		for i := range out {
			if n := names[out[i].ProviderID]; n != "" {
				out[i].ProviderName = n
			}
		}
	}
	return out, nil
}

// AggregateUsage — daily/per-provider/per-model aggregate untuk Usage tab.
type UsageRow struct {
	Day              string  `json:"day"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	RequestCount     int64   `json:"requestCount"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	CostUsd          float64 `json:"costUsd"`
}

func AggregateUsage(d *sql.DB, fromDay, toDay string) ([]UsageRow, error) {
	q := `SELECT day, provider, model, requestCount, promptTokens, completionTokens, costUsd
		FROM usageDaily WHERE 1=1`
	args := []any{}
	if fromDay != "" {
		q += ` AND day >= ?`
		args = append(args, fromDay)
	}
	if toDay != "" {
		q += ` AND day <= ?`
		args = append(args, toDay)
	}
	q += ` ORDER BY day DESC, costUsd DESC LIMIT 500`
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageRow
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.Day, &r.Provider, &r.Model, &r.RequestCount,
			&r.PromptTokens, &r.CompletionTokens, &r.CostUsd); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// TodayTotals — quick summary buat dashboard.
type TodayTotals struct {
	Day              string  `json:"day"`
	RequestCount     int64   `json:"requestCount"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	CostUsd          float64 `json:"costUsd"`
}

func TodaySummary(d *sql.DB) (*TodayTotals, error) {
	day := time.Now().UTC().Format("2006-01-02")
	row := d.QueryRow(`SELECT
		COALESCE(SUM(requestCount), 0),
		COALESCE(SUM(promptTokens), 0),
		COALESCE(SUM(completionTokens), 0),
		COALESCE(SUM(costUsd), 0)
		FROM usageDaily WHERE day = ?`, day)
	t := TodayTotals{Day: day}
	if err := row.Scan(&t.RequestCount, &t.PromptTokens, &t.CompletionTokens, &t.CostUsd); err != nil {
		return nil, err
	}
	return &t, nil
}
