// OpenAI Codex (ChatGPT backend) usage.
// GET https://chatgpt.com/backend-api/wham/usage with Bearer token.
// Response carries rate_limit / rate_limits_by_limit_id with reset windows.

package quotalive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() { Register(&codexFetcher{}) }

type codexFetcher struct{}

func (c *codexFetcher) Name() string { return "codex" }

func (c *codexFetcher) Fetch(ctx context.Context, p Params) (Snapshot, error) {
	if p.Token == "" {
		return Snapshot{}, fmt.Errorf("codex: bearer token required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://chatgpt.com/backend-api/wham/usage", nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("build req: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Snapshot{}, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Snapshot{}, fmt.Errorf("codex %d: %s", resp.StatusCode, snip(body))
	}

	var parsed struct {
		PlanType   string         `json:"plan_type,omitempty"`
		RateLimit  map[string]any `json:"rate_limit,omitempty"`
		RateLimits map[string]any `json:"rate_limits,omitempty"`
		Summary    struct {
			Plan string `json:"plan,omitempty"`
		} `json:"summary,omitempty"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Snapshot{}, fmt.Errorf("parse: %w", err)
	}

	snap := Snapshot{
		Provider:  "codex",
		Plan:      firstNonEmpty(parsed.PlanType, parsed.Summary.Plan),
		FetchedAt: time.Now().UTC(),
		Raw:       body,
	}
	// Codex's rate_limit shape varies; surface a single "window" with the
	// numeric `limit` / `used` / `reset_at` fields when present.
	src := parsed.RateLimit
	if len(src) == 0 {
		src = parsed.RateLimits
	}
	if len(src) > 0 {
		used := toFloat(src["used"])
		total := toFloat(src["limit"])
		remaining := total - used
		if remaining < 0 {
			remaining = 0
		}
		rp := 0.0
		if total > 0 {
			rp = (remaining / total) * 100
		}
		win := Window{
			Label:            "primary",
			Used:             used,
			Total:            total,
			Remaining:        remaining,
			RemainingPercent: rp,
			Unit:             "requests",
		}
		if s, _ := src["reset_at"].(string); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				win.ResetAt = t
			}
		}
		snap.Windows = append(snap.Windows, win)
	}
	return snap, nil
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	}
	return 0
}
