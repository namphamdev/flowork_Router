package provider

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/teetah2402/flowork/internal/safeclient"
)

// llmHTTPClient — used for non-streaming LLM completions. EXTBUG-021:
// previously returned a raw *http.Client with no SSRF guard, so a config
// override of `provider.base_url` (or any direct write bypassing the
// blocklist) could route every LLM call — Bearer API key attached — to
// an attacker-controlled endpoint. safeclient.NewClient installs the
// SSRF dialer + TLS 1.2+ defaults.
func llmHTTPClient() *http.Client {
	return safeclient.NewClient(10 * time.Minute)
}

// streamHTTPClient — used for SSE/streaming responses.
// No global timeout: the stream lasts as long as the model needs.
// Cancellation is handled via the request's context. Safe dialer still
// applies so streaming can't be aimed at a metadata endpoint either.
func streamHTTPClient() *http.Client {
	return safeclient.NewClient(0)
}

// postJSON mengirim JSON POST request dan mem-parse respons JSON.
func postJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, requestBody any, responseBody any) error {
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	if err := json.Unmarshal(body, responseBody); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// retryPostJSON — same as postJSON but retries up to maxRetries times on
// transient errors (network timeout, connection reset, EOF, HTTP 5xx).
// The caller context is checked before each retry so the loop exits cleanly
// when the caller cancels.
func retryPostJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, requestBody any, responseBody any) error {
	const maxRetries = 3
	backoff := 3 * time.Second
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// EXTBUG-029 fix: apply ±20% jitter so N flowork instances
			// hitting the same rate limit don't form a thundering herd that
			// all retry at identical wall-clock ticks.
			wait := jitter(backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			backoff *= 2 // 3s → 6s → 12s
		}
		err := postJSON(ctx, client, url, headers, requestBody, responseBody)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isTransientHTTPError(err) {
			return err // non-retryable (auth error, bad request, etc.)
		}
		// 429/quota — let FallbackClient switch to next tier instead of waiting
		// 3+6+12s retry here. In-place retry only makes sense untuk network
		// glitch; untuk rate-limit, pindah tier lebih cepat + lebih reliable.
		if isRateLimitError(err) {
			return err
		}
	}
	return fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

// isRateLimitError matches 429 / rate limit / quota errors. Stricter subset
// dari isTransientHTTPError — pemisahan ini bikin retry-in-place skip cepat
// buat 429 (biar FallbackClient ambil alih) tapi tetap retry untuk network.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "provider returned 429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "quota exceeded") ||
		strings.Contains(msg, "too many requests")
}

// jitter returns d scaled by a random factor in [0.8, 1.2] so concurrent
// retry storms desync. Falls back to d on RNG failure (never blocks retry).
func jitter(d time.Duration) time.Duration {
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(40))
	if err != nil {
		return d
	}
	// factor in [80..120] percent
	factor := 80 + n.Int64()
	return time.Duration(int64(d) * factor / 100)
}

// isTransientHTTPError returns true for errors that are safe to retry.
// Covers network errors, HTTP 5xx, and HTTP 429 (rate limit / quota exceeded).
func isTransientHTTPError(err error) bool {
	if err == nil {
		return false
	}
	// Don't retry if the caller's context was cancelled.
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		// Tightened: match "eof" only at word boundary to avoid false
		// positives on words like "thereof", "whereof", etc.
		strings.HasSuffix(msg, "eof") ||
		strings.Contains(msg, "eof:") ||
		strings.Contains(msg, "eof)") ||
		strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "temporarily unavailable") ||
		strings.Contains(msg, "provider returned 5") || // HTTP 5xx
		strings.Contains(msg, "provider returned 429") || // HTTP 429 rate limit
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "quota exceeded") ||
		strings.Contains(msg, "too many requests")
}
