// Outbound Proxy Selection (Proxy Pools → Dispatch).

package router

import (
	"context"
	"hash/fnv"
	"math/rand"
	"net/http"
	"net/url"
	"sync"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// clientCache memoizes one *http.Client per proxy URL so connection pools are
// reused across requests — building a Transport per call would defeat
// keep-alive. proxyCursor holds the round-robin index per pool ID.
var (
	proxyMu     sync.Mutex
	clientCache = map[string]*http.Client{}
	proxyCursor = map[string]int{}
)

// outboundClient returns the HTTP client for upstream provider calls.
// Resolution order:
//
//  1. Active DB-configured proxy pool — wins so operators can pin egress IPs.
//  2. Plain httpClient — uses Go's http.DefaultTransport which honours the
//     standard HTTP_PROXY / HTTPS_PROXY / NO_PROXY environment variables
//     out of the box (no extra code required). Set HTTP_PROXY before
//     starting flow_router to route all upstream traffic through a
//     corporate proxy; set NO_PROXY to whitelist hosts that should bypass.
//
// ctx carries the client identity used for sticky affinity within a pool.
func outboundClient(ctx context.Context) *http.Client {
	if u := pickProxyURL(ctx); u != "" {
		return clientForProxy(u)
	}
	return httpClient
}

// OutboundClient is the exported entry so non-router packages (e.g. the media
// dispatch handlers) route their upstream egress through proxy pools too —
// otherwise media requests (which carry user content) would bypass the proxy.
func OutboundClient(ctx context.Context) *http.Client { return outboundClient(ctx) }

// pickProxyURL returns a proxy URL from the first active pool per its rotation
// strategy, or "" when there is no usable pool.
func pickProxyURL(ctx context.Context) string {
	d, err := store.Open()
	if err != nil {
		return ""
	}
	pools, err := store.ListProxyPools(d)
	if err != nil {
		return ""
	}
	for _, p := range pools {
		if !p.IsActive || len(p.Proxies) == 0 {
			continue
		}
		switch p.Rotation {
		case store.ProxyRotationRandom:
			return p.Proxies[rand.Intn(len(p.Proxies))]
		case store.ProxyRotationSticky:
			// Affinity: the same client always maps to the same proxy (so a
			// session keeps one egress IP). Key by client IP, falling back to
			// the API key, then proxy[0] when neither is known.
			key := clientIdentity(ctx)
			if key == "" {
				return p.Proxies[0]
			}
			h := fnv.New32a()
			_, _ = h.Write([]byte(key))
			return p.Proxies[int(h.Sum32())%len(p.Proxies)]
		default: // round-robin
			proxyMu.Lock()
			i := proxyCursor[p.ID] % len(p.Proxies)
			proxyCursor[p.ID] = (i + 1) % len(p.Proxies)
			proxyMu.Unlock()
			return p.Proxies[i]
		}
	}
	return ""
}

// clientForProxy returns a cached proxied client for proxyURL, falling back to
// the direct client if the URL cannot be parsed.
func clientForProxy(proxyURL string) *http.Client {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	if c, ok := clientCache[proxyURL]; ok {
		return c
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return httpClient
	}
	c := &http.Client{
		Timeout:   httpTimeout,
		Transport: &http.Transport{Proxy: http.ProxyURL(u)},
	}
	clientCache[proxyURL] = c
	return c
}
