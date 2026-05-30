// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flow_router
// Locked at: 2026-05-30
// Reason: Section 26 phase 2 real-time pricing calc dari pricing_rules
//   table. Caller (chat handler post-response middleware) panggil Calc
//   buat cost_usd, lalu LogCall buat persist ke provider_call_log.
//   Phase 3 (multi-tier dynamic resolve per warga, bulk import OpenRouter)
//   → tambah file baru.
//
// calc.go — Section 26 phase 2: real-time cost calc + call log writer.

package pricing

import (
	"database/sql"
	"time"
)

// Calc — return cost_usd untuk (provider, model, tier). Tier default
// 'default' kalau kosong. Cost = (input/1e6 × input_rate) + (output/1e6 × output_rate).
//
// Kalau pricing_rules ngga ada row, return 0 (graceful no-cost).
func Calc(db *sql.DB, provider, model, tier string, inputToks, outputToks int) (float64, error) {
	if tier == "" {
		tier = "default"
	}
	var inputRate, outputRate float64
	err := db.QueryRow(
		`SELECT input_per_1m_usd, output_per_1m_usd
		 FROM pricing_rules
		 WHERE provider = ? AND model = ? AND tier = ? AND enabled = 1`,
		provider, model, tier).Scan(&inputRate, &outputRate)
	if err == sql.ErrNoRows {
		// Fallback: query tanpa tier specific.
		err = db.QueryRow(
			`SELECT input_per_1m_usd, output_per_1m_usd
			 FROM pricing_rules
			 WHERE provider = ? AND model = ? AND enabled = 1
			 ORDER BY (CASE WHEN tier = 'default' THEN 0 ELSE 1 END), id
			 LIMIT 1`,
			provider, model).Scan(&inputRate, &outputRate)
		if err == sql.ErrNoRows {
			return 0, nil
		}
	}
	if err != nil {
		return 0, err
	}
	inputUSD := float64(inputToks) / 1_000_000.0 * inputRate
	outputUSD := float64(outputToks) / 1_000_000.0 * outputRate
	return inputUSD + outputUSD, nil
}

// LogCall — append row ke provider_call_log. Caller pass cost_usd
// (biasanya dari Calc) + latency_ms + status.
func LogCall(db *sql.DB, caller, provider, model string, inputToks, outputToks int,
	costUSD float64, latencyMS int64, status string) error {
	if status == "" {
		status = "success"
	}
	_, err := db.Exec(
		`INSERT INTO provider_call_log
		   (caller, provider, model, input_tokens, output_tokens,
		    cost_usd, latency_ms, status, occurred_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		caller, provider, model, inputToks, outputToks,
		costUSD, latencyMS, status,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}
