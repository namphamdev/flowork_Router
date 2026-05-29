// Package mesh — karma_decay.go: weekly karma decay cron (M5 Step 2).
//
// Per M05-karma-scoring.md §Step 2:
// Karma drift toward 0.5 perlahan kalau no activity (anti-stale-trust).
// Per minggu: karma_new = karma_old * 0.99 + 0.5 * 0.01.
//
// Cron mingguan di kernel — started at boot, runs forever.
package mesh

import (
	"context"
	"log"
	"time"
)

// StartKarmaDecayCron — start weekly background goroutine that decays all
// peer karma toward neutral (0.5). Stops when ctx cancelled.
//
// Tick interval is 7 days. For testing, use StartKarmaDecayWithInterval.
func StartKarmaDecayCron(ctx context.Context, engine *KarmaEngine) {
	StartKarmaDecayWithInterval(ctx, engine, 7*24*time.Hour)
}

// StartKarmaDecayWithInterval — same as StartKarmaDecayCron but with
// configurable interval (for testing).
func StartKarmaDecayWithInterval(ctx context.Context, engine *KarmaEngine, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := engine.WeeklyDecay(); err != nil {
					log.Printf("[karma] decay error: %v", err)
				} else {
					log.Printf("[karma] weekly decay applied to %d peers", len(engine.AllPeers()))
				}
			}
		}
	}()
}
