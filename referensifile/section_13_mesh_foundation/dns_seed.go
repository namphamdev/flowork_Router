// Package mesh — dns_seed.go: DNS TXT record bootstrap untuk peer discovery.
//
// Phase C step 6 per VISI_FINAL Pilar 3:
//
//	"Bootstrap: DNS seeds (e.g., seed.flowork.app — TXT record berisi list
//	 peer initial)"
//
// Pattern Bitcoin-style: kalau settings DB key MESH_PEER_SEEDS empty, kernel
// fallback ke DNS lookup. Domain bisa di-override via env atau settings DB
// key MESH_DNS_SEED_DOMAIN. Default: "seed.flowork.app".
//
// TXT record format (multiple allowed, di-merge):
//
//	seed.flowork.app  TXT  "https://node-jakarta.flowork.ayah:3105,https://node-sg.flowork.ayah:3105"
//	seed.flowork.app  TXT  "https://relay.flowork.app:443"
//
// Tiap TXT value di-parse sebagai comma-separated URL. Sanitize + dedupe
// sama kayak Discover() di peer_discover.go.
//
// Ngga ada signature di TXT record (DNS itself ngga ditrust), jadi peer
// hasil bootstrap masuk ke trust score default (50, neutral). Identity
// handshake nanti yang verify legitimacy via pubkey.

package mesh

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/flowork/kernel/kernel/settings"
)

const (
	settingsKeyDNSSeedDomain = "MESH_DNS_SEED_DOMAIN"
	defaultSeedDomain        = "seed.flowork.app"
	dnsLookupTimeout         = 5 * time.Second
)

// txtResolver — small interface untuk testability.
//
// net.DefaultResolver memenuhi via LookupTXT(ctx, domain) (dari Go 1.15+).
// Test pake fake resolver.
type txtResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// defaultResolver wraps Go's net.DefaultResolver dengan timeout.
var defaultResolver txtResolver = &net.Resolver{
	PreferGo: true,
}

// DiscoverViaDNS — query TXT records dari domain seed (settings DB
// MESH_DNS_SEED_DOMAIN, default "seed.flowork.app"), parse jadi peer list.
//
// Empty result + nil error = ngga ada TXT record (bukan failure mode —
// caller fallback ke single-node).
//
// 5s timeout. Cuma dipanggil saat MESH_PEER_SEEDS empty, jadi ngga add
// latency ke happy path.
func DiscoverViaDNS(ctx context.Context) ([]Peer, error) {
	domain := defaultSeedDomain
	if store := settings.Shared(); store != nil {
		if v, _ := store.Get(settingsKeyDNSSeedDomain); strings.TrimSpace(v) != "" {
			domain = strings.TrimSpace(v)
		}
	}
	return discoverViaDNSWithResolver(ctx, defaultResolver, domain)
}

// discoverViaDNSWithResolver — internal helper yang accept resolver
// interface untuk unit testing.
func discoverViaDNSWithResolver(ctx context.Context, r txtResolver, domain string) ([]Peer, error) {
	if domain == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, dnsLookupTimeout)
	defer cancel()

	records, err := r.LookupTXT(ctx, domain)
	if err != nil {
		// DNS NXDOMAIN / timeout — ngga propagate error fatal, single-node OK.
		// Caller log warning saja.
		return nil, err
	}

	seen := make(map[string]bool)
	out := make([]Peer, 0, len(records))
	for _, rec := range records {
		// Tiap TXT record bisa berisi multiple URL comma-separated.
		for _, raw := range strings.Split(rec, ",") {
			u := strings.TrimRight(strings.TrimSpace(raw), "/")
			if u == "" || seen[u] {
				continue
			}
			seen[u] = true
			out = append(out, Peer{URL: u, Tags: []string{"dns-seed"}})
		}
	}
	return out, nil
}

// DiscoverWithFallback — primary entry point: try MESH_PEER_SEEDS dulu,
// fallback DNS kalau empty. Cocok dipanggil dari kernel boot atau periodic
// peer refresh.
func DiscoverWithFallback(ctx context.Context) ([]Peer, error) {
	peers, err := Discover()
	if err == nil && len(peers) > 0 {
		return peers, nil
	}
	// Fallback DNS.
	return DiscoverViaDNS(ctx)
}
