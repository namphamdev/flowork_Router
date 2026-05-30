// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Router dispatcher.

// Inbound API-Key Context (auth → dispatch bridge).

package router

import (
	"context"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/store"
)

type ctxKeyAPIKeyType struct{}

var ctxKeyAPIKey = ctxKeyAPIKeyType{}

// WithAPIKey stashes a validated inbound API key in ctx.
func WithAPIKey(ctx context.Context, k *store.APIKey) context.Context {
	return context.WithValue(ctx, ctxKeyAPIKey, k)
}

// APIKeyFromContext returns the validated key, or nil for anonymous/open mode.
func APIKeyFromContext(ctx context.Context) *store.APIKey {
	k, _ := ctx.Value(ctxKeyAPIKey).(*store.APIKey)
	return k
}

// filterByAllowedProviders keeps only providers the key is permitted to use.
// "*" or empty = all providers. A provider matches when its type (p.Provider,
// e.g. "anthropic") OR its display Name is in the key's CSV allow-list,
// case-insensitive. nil key = no restriction.
func filterByAllowedProviders(matches []store.ProviderConnection, k *store.APIKey) []store.ProviderConnection {
	if k == nil {
		return matches
	}
	allow := strings.TrimSpace(k.AllowedProviders)
	if allow == "" || allow == "*" {
		return matches
	}
	allowed := map[string]bool{}
	for _, a := range strings.Split(allow, ",") {
		if a = strings.ToLower(strings.TrimSpace(a)); a != "" {
			allowed[a] = true
		}
	}
	var out []store.ProviderConnection
	for _, p := range matches {
		if allowed[strings.ToLower(p.Provider)] || allowed[strings.ToLower(p.Name)] {
			out = append(out, p)
		}
	}
	return out
}

// apiKeyID returns the key's ID, or "" when anonymous.
func apiKeyID(ctx context.Context) string {
	if k := APIKeyFromContext(ctx); k != nil {
		return k.ID
	}
	return ""
}

type ctxKeyClientIPType struct{}

var ctxKeyClientIP = ctxKeyClientIPType{}

// WithClientIP stashes the inbound client IP for sticky proxy affinity.
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ctxKeyClientIP, ip)
}

// clientIdentity returns a stable per-client key for sticky routing: the client
// IP if known, else the API key ID, else "".
func clientIdentity(ctx context.Context) string {
	if ip, _ := ctx.Value(ctxKeyClientIP).(string); ip != "" {
		return ip
	}
	return apiKeyID(ctx)
}
