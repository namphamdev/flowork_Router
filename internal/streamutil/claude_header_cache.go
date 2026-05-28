// Claude Header Cache (forward real CLI identity).

package streamutil

import (
	"strings"
	"sync"
	"sync/atomic"
)

// ClaudeIdentityHeaders mirrors upstream's allow-list — only these headers are
// copied to the cache to avoid leaking client IPs, auth tokens, etc.
var ClaudeIdentityHeaders = []string{
	"user-agent",
	"anthropic-beta",
	"anthropic-version",
	"anthropic-dangerous-direct-browser-access",
	"x-app",
	"x-stainless-helper-method",
	"x-stainless-retry-count",
	"x-stainless-runtime-version",
	"x-stainless-package-version",
	"x-stainless-runtime",
	"x-stainless-lang",
	"x-stainless-arch",
	"x-stainless-os",
	"x-stainless-timeout",
	"x-claude-code-session-id",
	"package-version",
	"runtime-version",
	"os",
	"arch",
}

var (
	cachedHeaders atomic.Value // map[string]string
	cacheMu       sync.Mutex
)

// IsClaudeCodeClient sniffs the request headers (lowercased keys) for the
// signature of a real Claude Code / Claude CLI session.
func IsClaudeCodeClient(headers map[string]string) bool {
	ua := strings.ToLower(headers["user-agent"])
	xApp := strings.ToLower(headers["x-app"])
	return strings.Contains(ua, "claude-cli") || strings.Contains(ua, "claude-code") || xApp == "cli"
}

// CaptureFromRequest grabs the identity headers from headers (lowercased
// keys) and stores them as the new cache snapshot. No-op when the request
// does not look like a Claude Code client.
func CaptureFromRequest(headers map[string]string) {
	if !IsClaudeCodeClient(headers) {
		return
	}
	snapshot := map[string]string{}
	for _, k := range ClaudeIdentityHeaders {
		if v := headers[strings.ToLower(k)]; v != "" {
			snapshot[k] = v
		}
	}
	if len(snapshot) == 0 {
		return
	}
	cacheMu.Lock()
	cachedHeaders.Store(snapshot)
	cacheMu.Unlock()
}

// GetCachedClaudeHeaders returns the latest captured headers, or nil when
// nothing has been cached yet.
func GetCachedClaudeHeaders() map[string]string {
	v := cachedHeaders.Load()
	if v == nil {
		return nil
	}
	m, _ := v.(map[string]string)
	if m == nil {
		return nil
	}
	// Return a copy to keep the snapshot immutable.
	out := make(map[string]string, len(m))
	for k, vv := range m {
		out[k] = vv
	}
	return out
}

// HasCachedClaudeHeaders reports whether at least one snapshot has been stored.
func HasCachedClaudeHeaders() bool { return cachedHeaders.Load() != nil }
