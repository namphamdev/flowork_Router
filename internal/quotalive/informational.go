// Informational fetchers: vendors that don't expose a public quota / usage
// endpoint. We register them anyway so the live-quota panel can render a
// "connected, no public API" entry instead of "not implemented".

package quotalive

import (
	"context"
	"fmt"
	"time"
)

func init() {
	Register(&informationalFetcher{
		vendor:  "iflow",
		message: "iFlow connected. No public usage API — quota tracked per request.",
	})
	Register(&informationalFetcher{
		vendor:  "qwen",
		message: "Qwen connected. No public usage API — quota tracked per request.",
	})
	Register(&informationalFetcher{
		vendor:  "ollama",
		message: "Ollama Cloud connected. No public usage API — free tier limits reset every 5h & 7d.",
	})
}

// informationalFetcher returns a fixed message Snapshot. The token is still
// required so the UI doesn't render the entry until the user actually has
// credentials configured.
type informationalFetcher struct {
	vendor  string
	message string
}

func (f *informationalFetcher) Name() string { return f.vendor }

func (f *informationalFetcher) Fetch(ctx context.Context, p Params) (Snapshot, error) {
	if p.Token == "" {
		return Snapshot{}, fmt.Errorf("%s: token required", f.vendor)
	}
	return Snapshot{
		Provider:  f.vendor,
		Plan:      f.message,
		FetchedAt: time.Now().UTC(),
		// Windows intentionally empty — vendor doesn't expose quota.
	}, nil
}
