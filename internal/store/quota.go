// Quota Tracker.

package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// QuotaStatus — per-provider quota snapshot.
type QuotaStatus struct {
	ProviderID       string `json:"providerId"`
	ProviderName     string `json:"providerName"`
	Provider         string `json:"provider"` // type (anthropic, openai, etc)
	AuthType         string `json:"authType"`
	SubscriptionTier string `json:"subscriptionTier,omitempty"`

	TodayRequests  int64   `json:"todayRequests"`
	TodayPromptTok int64   `json:"todayPromptTok"`
	TodayComplTok  int64   `json:"todayComplTok"`
	TodayCostUsd   float64 `json:"todayCostUsd"`

	WeekRequests  int64   `json:"weekRequests"`
	WeekPromptTok int64   `json:"weekPromptTok"`
	WeekComplTok  int64   `json:"weekComplTok"`
	WeekCostUsd   float64 `json:"weekCostUsd"`

	MonthRequests int64   `json:"monthRequests"`
	MonthCostUsd  float64 `json:"monthCostUsd"`

	// Quota is DERIVED from local usage (same approach as upstream): there is no
	// live provider-side quota poll because the supported providers don't expose
	// one (Anthropic subscription / API keys have no usage-quota endpoint). ResetAt
	// comes from a provider's optional `quotaResetHours` config; HealthOk = active.
	ResetAt  string `json:"resetAt,omitempty"`
	HealthOk bool   `json:"healthOk"`
}

// ListQuotaStatus — return quota summary per active provider.
// Derive dari usageDaily table (Phase 1.5). Live quota poll = Phase 2.
func ListQuotaStatus(d *sql.DB) ([]QuotaStatus, error) {
	providers, err := ListProviders(d)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	now := time.Now().UTC()
	todayStr := now.Format("2006-01-02")
	weekAgo := now.AddDate(0, 0, -7).Format("2006-01-02")
	monthAgo := now.AddDate(0, -1, 0).Format("2006-01-02")

	var out []QuotaStatus
	for _, p := range providers {
		q := QuotaStatus{
			ProviderID:   p.ID,
			ProviderName: p.Name,
			Provider:     p.Provider,
			AuthType:     p.AuthType,
			HealthOk:     p.IsActive,
		}
		if tier, ok := p.Data["subscriptionType"].(string); ok {
			q.SubscriptionTier = tier
		}

		// Today
		row := d.QueryRow(`SELECT
			COALESCE(SUM(requestCount), 0),
			COALESCE(SUM(promptTokens), 0),
			COALESCE(SUM(completionTokens), 0),
			COALESCE(SUM(costUsd), 0)
			FROM usageDaily WHERE day = ? AND provider = ?`, todayStr, p.ID)
		_ = row.Scan(&q.TodayRequests, &q.TodayPromptTok, &q.TodayComplTok, &q.TodayCostUsd)

		// Week (last 7 days)
		row = d.QueryRow(`SELECT
			COALESCE(SUM(requestCount), 0),
			COALESCE(SUM(promptTokens), 0),
			COALESCE(SUM(completionTokens), 0),
			COALESCE(SUM(costUsd), 0)
			FROM usageDaily WHERE day >= ? AND provider = ?`, weekAgo, p.ID)
		_ = row.Scan(&q.WeekRequests, &q.WeekPromptTok, &q.WeekComplTok, &q.WeekCostUsd)

		// Month (last 30 days)
		row = d.QueryRow(`SELECT
			COALESCE(SUM(requestCount), 0),
			COALESCE(SUM(costUsd), 0)
			FROM usageDaily WHERE day >= ? AND provider = ?`, monthAgo, p.ID)
		_ = row.Scan(&q.MonthRequests, &q.MonthCostUsd)

		// Reset window is DATA-DRIVEN: a provider may declare a rolling quota
		// window length (hours) in its config as `quotaResetHours`. No provider
		// names or magic numbers hardcoded. Unset → no reset (e.g. API keys).
		if h := quotaResetHours(p.Data["quotaResetHours"]); h > 0 {
			win := time.Duration(h * float64(time.Hour))
			q.ResetAt = now.Add(win).Truncate(win).Format(time.RFC3339)
		}

		out = append(out, q)
	}
	return out, nil
}

// quotaResetHours coerces the config value (JSON number or string) to hours.
func quotaResetHours(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}
