// Fallback chain — wraps an ordered list of Client implementations and
// tries each in turn when the previous one errors in a way we consider
// recoverable (timeout, network failure, 5xx, 429, quota, deadline).
//
// Intent: keep FLOWORK thinking even when the primary API is down. Typical
// chain:
//
//	DeepSeek (cloud, cheapest)  →  gemini-cli (user subscription)  →  Ollama (offline)
//
// Each client gets a short circuit-breaker: after N consecutive failures the
// client is skipped for cooldown seconds. Prevents a dead provider from
// eating the full timeout budget on every call.
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FallbackClient is a Client that dispatches to the first non-broken
// provider in a chain.
type FallbackClient struct {
	chain []*circuitBreaker
	name  string
}

// FallbackConfig tunes the circuit breaker.
type FallbackConfig struct {
	// FailThreshold — consecutive failures before the breaker opens. Default 3.
	FailThreshold int
	// Cooldown — time to wait before probing an open breaker again. Default 30s.
	Cooldown time.Duration
}

// NewFallbackClient wraps one or more providers. Order matters: the first
// client is tried first, last is the ultimate fallback.
//
// BUG-C02 fix (2026-04-19): no longer panics on empty clients. Returns a
// degenerate FallbackClient whose Complete() always returns a clear error.
// This lets caller handle the error gracefully instead of crashing startup.
func NewFallbackClient(cfg FallbackConfig, clients ...Client) *FallbackClient {
	if len(clients) == 0 {
		return &FallbackClient{
			chain: nil,
			name:  "fallback[empty]",
		}
	}
	if cfg.FailThreshold <= 0 {
		cfg.FailThreshold = 3
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = 30 * time.Second
	}
	chain := make([]*circuitBreaker, len(clients))
	names := make([]string, len(clients))
	for i, c := range clients {
		chain[i] = &circuitBreaker{
			client:    c,
			threshold: cfg.FailThreshold,
			cooldown:  cfg.Cooldown,
		}
		names[i] = c.Name()
	}
	return &FallbackClient{
		chain: chain,
		name:  "fallback[" + strings.Join(names, ">") + "]",
	}
}

// Name returns the provider identifier (includes inner chain labels).
func (f *FallbackClient) Name() string { return f.name }

// Complete walks the chain until one client succeeds or all are broken.
// Returns the first non-recoverable error verbatim so callers can see the
// original reason (auth failure, bad request, etc.) instead of a generic
// "all providers failed".
func (f *FallbackClient) Complete(ctx context.Context, req Request) (Response, error) {
	// BUG-C02 fix: fail gracefully when chain is empty (zero-clients init).
	if len(f.chain) == 0 {
		return Response{}, fmt.Errorf("provider.FallbackClient: no providers configured")
	}
	var firstErr error
	var fatalErr error // from any client that returned non-recoverable
	for _, cb := range f.chain {
		if !cb.canTry() {
			continue
		}
		// BUG-14: wrap the call so a panic inside cb.client.Complete can't
		// leave probesBusy=true permanently (half-open breaker stuck forever).
		resp, err := func() (r Response, e error) {
			defer func() {
				if rec := recover(); rec != nil {
					cb.recordFailure()
					e = fmt.Errorf("panic in %s: %v", cb.client.Name(), rec)
				}
			}()
			return cb.client.Complete(ctx, req)
		}()
		if err == nil {
			cb.recordSuccess()
			return resp, nil
		}
		// If the caller cancelled, bail out — not the chain's problem.
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		if !isRecoverable(err) {
			// Gemini audit Bug 6.4 fix: if the error is REQUEST-SPECIFIC
			// (malformed payload / schema / invalid model), bail immediately
			// instead of spamming every provider with a payload guaranteed
			// to fail. Auth/quota errors CAN still try next provider —
			// different creds might work.
			if IsFatalError(err) {
				cb.recordFailure()
				return Response{}, fmt.Errorf("%s (fatal, skipping remaining providers): %w", cb.client.Name(), err)
			}
			// Non-recoverable but not fatal (e.g. 401 auth on one provider):
			// remember but keep trying — next provider may have fresh creds.
			if fatalErr == nil {
				fatalErr = fmt.Errorf("%s: %w", cb.client.Name(), err)
			}
			cb.recordFailure()
			continue
		}
		// Ayah req 2026-04-24: rate-limit (429) pada free tier itu HAL BIASA —
		// jangan trigger circuit breaker cooldown (bikin semua tier tidur
		// bareng 15 menit). Cukup pindah tier next, breaker state biarin.
		// Cooldown tetap aktif buat error transien lain (5xx, timeout) yang
		// beneran indikasi provider down.
		if !isRateLimitError(err) {
			cb.recordFailure()
		}
		if firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", cb.client.Name(), err)
		}
	}
	// Prioritize fatal errors over recoverable ones when both occurred.
	// A fatal error (auth, bad request) almost always means a real
	// misconfiguration that the caller MUST see — masking it behind
	// a transient timeout from another provider hides the bug.
	// Audit by progem (2026-04-15): error-priority swap.
	if fatalErr != nil {
		if firstErr != nil {
			return Response{}, fmt.Errorf("%w (also: %s)", fatalErr, firstErr)
		}
		return Response{}, fatalErr
	}
	if firstErr != nil {
		return Response{}, fmt.Errorf("all providers failed; first error: %w", firstErr)
	}
	return Response{}, errors.New("all providers are in cooldown")
}

// ─── Circuit breaker ───────────────────────────────────────────────

type circuitBreaker struct {
	client    Client
	threshold int
	cooldown  time.Duration

	mu          sync.Mutex
	consecutive int
	openedAt    time.Time
	probesBusy  atomic.Bool // true while a probe call is in-flight (half-open)
}

// canTry reports whether the breaker allows a call through. Open breakers
// permit one probe per cooldown window (half-open pattern).
func (b *circuitBreaker) canTry() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.consecutive < b.threshold {
		return true // closed
	}
	if time.Since(b.openedAt) < b.cooldown {
		return false // open, still cooling
	}
	// half-open: allow exactly one probe at a time
	if b.probesBusy.CompareAndSwap(false, true) {
		return true
	}
	return false
}

func (b *circuitBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutive = 0
	b.openedAt = time.Time{}
	b.probesBusy.Store(false)
}

func (b *circuitBreaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutive++
	if b.consecutive >= b.threshold && b.openedAt.IsZero() {
		b.openedAt = time.Now()
	}
	b.probesBusy.Store(false)
}

// IsRecoverable reports whether an error is transient — rate limit, token /
// quota exhaustion, timeout, 5xx, or network blip. Callers outside the
// fallback chain (e.g. the agent loop's 15-minute wait-and-retry) use this
// to decide whether to sleep and try again vs. surface the error.
func IsRecoverable(err error) bool { return isRecoverable(err) }

// isRecoverable returns true for errors we should retry against another
// provider: timeouts, transient network issues, rate limits, 5xx responses.
// Auth errors, malformed requests, etc. are not — they'd fail the same way
// on the next provider (or signal a bug worth surfacing immediately).
func isRecoverable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	// rc177 fix: 402 Payment Required / account-balance-exhausted MUST be
	// classified non-recoverable BEFORE generic "quota" match below.
	// OpenRouter 402 response body contains "insufficient_quota" yang previously
	// matched strings.Contains(msg, "quota") → IsRecoverable=true → agent
	// loop retry 15-min × N → wall-clock 30m timeout. Real bug dari
	// Ayah's floworkos bot laporan 2026-04-20.
	//
	// Semantic distinction:
	//   - "rate limit quota" (429) = transient, retry OK
	//   - "account balance quota" (402) = permanent until refill, retry useless
	// Priority-check 402 first supaya short-circuit benar.
	if strings.Contains(msg, "402") || strings.Contains(msg, "payment required") ||
		strings.Contains(msg, "insufficient_quota") || strings.Contains(msg, "insufficient credits") ||
		strings.Contains(msg, "insufficient balance") {
		return false
	}
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests") || strings.Contains(msg, "quota") {
		return true
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection reset") || strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "broken pipe") || strings.Contains(msg, "temporarily unavailable") {
		return true
	}
	// 5xx responses from our providers surface as "provider returned 5xx: ...".
	if strings.Contains(msg, "provider returned 5") ||
		strings.Contains(msg, "bridge returned 5") ||
		strings.Contains(msg, "ollama returned 5") {
		return true
	}
	// Service unreachable (network down, DNS failure).
	// BUG-H09 fix (2026-04-19): tambah pola DNS error Windows — getaddrinfow
	// dan varian "dial tcp: lookup ... no such host" yang tidak cukup match
	// hanya dengan substring "no such host" pada sebagian message Go runtime
	// di Windows 10/11.
	if strings.Contains(msg, "unreachable") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "getaddrinfow") ||
		strings.Contains(msg, "lookup ") ||
		strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "network is unreachable") {
		return true
	}
	// Prompt/context too long — reactive compaction: the agent's
	// completeWithRateLimitRetry will compact & retry automatically.
	if IsPromptTooLong(err) {
		return true
	}
	return false
}

// IsFatalError returns true for errors that are REQUEST-SPECIFIC, not
// provider-specific — meaning every provider in the fallback chain would
// fail the same way, so trying the next is wasted traffic/quota and just
// spams the logs with identical rejections.
//
// Gemini audit follow-up (Bug 6.4 from bug_core.md): previously the
// fallback chain kept going on non-recoverable errors (returned via
// `continue` in Complete loop), causing a malformed tool payload to hit
// every provider in sequence. Now, callers can bail out immediately when
// IsFatalError returns true.
//
// Conservative classification — we only mark as fatal the patterns that
// are demonstrably message-shape problems (schema/JSON/invalid params).
// Auth errors are NOT fatal here, because different providers have
// different API keys and the next one might have fresh credentials.
func IsFatalError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// HTTP 400 variants (Bad Request) — caller sent malformed payload.
	if strings.Contains(msg, "400 bad request") ||
		strings.Contains(msg, "http 400") ||
		strings.Contains(msg, "status 400") ||
		strings.Contains(msg, "returned 400") {
		return true
	}
	// JSON schema / validation errors — message shape is wrong, same for everyone.
	if strings.Contains(msg, "invalid request") ||
		strings.Contains(msg, "invalid_request_error") ||
		strings.Contains(msg, "invalid parameters") ||
		strings.Contains(msg, "invalid parameter") ||
		strings.Contains(msg, "schema validation") ||
		strings.Contains(msg, "unsupported parameter") ||
		strings.Contains(msg, "malformed") {
		return true
	}
	// Model-not-found — if the model name is typoed, no point retrying.
	if strings.Contains(msg, "model not found") ||
		strings.Contains(msg, "model_not_found") ||
		strings.Contains(msg, "invalid model") {
		return true
	}
	// 402 Payment Required / balance exhausted — affects ALL providers yang
	// share API key / billing account. Bailing immediately menghindari walk
	// melalui 11-tier chain yang semua akan 402 (~2.5 menit wasted).
	// Ayah 2026-04-24 insight: kalau balance habis, semua paid provider kena
	// error sama — ga ada gunanya retry tier berikutnya.
	if strings.Contains(msg, "402") || strings.Contains(msg, "payment required") ||
		strings.Contains(msg, "insufficient_quota") || strings.Contains(msg, "insufficient credits") ||
		strings.Contains(msg, "insufficient balance") {
		return true
	}
	return false
}

// IsPromptTooLong returns true if the error indicates the request exceeds
// the model's context window. Callers can use this to trigger reactive
// compaction before retrying (Claude Code parity: reactive compact).
func IsPromptTooLong(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "context length exceeded") ||
		strings.Contains(msg, "maximum context length") ||
		strings.Contains(msg, "request too large") ||
		strings.Contains(msg, "prompt too long") ||
		strings.Contains(msg, "maximum number of tokens")
}
