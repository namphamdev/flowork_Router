// flow_router Entry Point.

package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowork-os/flowork_Router/internal/store"

	// Side-effect import: each filter's init() registers itself with rtk.
	// New filters plug in by dropping a file under internal/rtk/filters/.
	_ "github.com/flowork-os/flowork_Router/internal/rtk/filters"

	// Side-effect imports: each translator pair self-registers via init().
	_ "github.com/flowork-os/flowork_Router/internal/translator/request"
	_ "github.com/flowork-os/flowork_Router/internal/translator/response"

	// Side-effect imports: each provider catalog file self-registers via init().
	_ "github.com/flowork-os/flowork_Router/internal/providers/embedding"
	_ "github.com/flowork-os/flowork_Router/internal/providers/image"
	_ "github.com/flowork-os/flowork_Router/internal/providers/tts"

	// Side-effect import: web-search vendor catalog (tavily/brave/serpapi/duckduckgo).
	_ "github.com/flowork-os/flowork_Router/internal/search"
)

//go:embed web/static
var webFS embed.FS

const version = "1.0.0-phase1.5-all-features-functional"

func main() {
	addr := flag.String("addr", "127.0.0.1:2402", "listen address")
	flag.Parse()

	log.Printf("flow_router %s starting on %s", version, *addr)
	log.Printf("Data: %s", store.DBPath())

	// Init storage + seed defaults
	d, err := store.Open()
	if err != nil {
		log.Fatalf("storage init: %v", err)
	}
	defer store.Close()
	if err := store.SeedDefaults(d); err != nil {
		log.Printf("WARN: seed defaults: %v", err)
	}
	if err := store.AugmentTierTags(d); err != nil {
		log.Printf("WARN: augment tier tags: %v", err)
	}
	if err := store.SeedDefaultPricing(d); err != nil {
		log.Printf("WARN: seed pricing: %v", err)
	}
	if err := store.PurgeExpiredSessions(d); err != nil {
		log.Printf("WARN: purge sessions: %v", err)
	}
	loadMITMCaptureState()
	startTunnelWatchdog()
	providers, _ := store.ListProviders(d)
	log.Printf("Providers loaded: %d", len(providers))
	for _, p := range providers {
		status := "off"
		if p.IsActive {
			status = "on"
		}
		log.Printf("  - [%s] %s (%s, priority=%d)", status, p.Name, p.AuthType, p.Priority)
	}

	mux := http.NewServeMux()

	// All HTTP routes live in routes.go, grouped per domain.
	registerRoutes(mux)

	srv := &http.Server{
		Addr: *addr,
		// Middleware chain (outermost first):
		//   apiKeyMiddleware    — gates /v1 + /v1beta with flow_router API keys
		//                         (opt-in via settings.RequireApiKey), enforces
		//                         per-key caps, injects the key into context.
		//   authEnforceMiddleware — OPT-IN GUI session gate (settings.RequireLogin);
		//                         exempts /v1, /api/auth, health, static, root.
		Handler: apiKeyMiddleware(authEnforceMiddleware(mux)),
		// Slowloris guard: caps on every phase of the HTTP transaction so a
		// stalled client cannot pin a server goroutine forever. ReadHeader is
		// the most important — request-line + headers must arrive in 15s.
		// Write/Idle are generous because /v1 streams completions for minutes.
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
		WriteTimeout:      10 * time.Minute,
	}

	// Graceful shutdown: /api/shutdown fires shutdownTriggerCh; SIGINT/SIGTERM
	// also closes the server cleanly.
	shutdownTriggerCh = make(chan struct{}, 1)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-shutdownTriggerCh:
			log.Printf("flow_router: shutdown requested via API")
		case s := <-sigCh:
			log.Printf("flow_router: signal %s received", s)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("flow_router serve error: %v", err)
		os.Exit(1)
	}
	log.Printf("flow_router stopped cleanly")
}
