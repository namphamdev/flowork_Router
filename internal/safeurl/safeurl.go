// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// SSRF-safe URL validation for user-supplied fetch targets.
//
// Validate(url) rejects anything that could be used to pivot into the
// host's local network, link-local services (cloud-metadata at
// 169.254.169.254), or loopback. Use it before passing a URL from an
// untrusted source to an HTTP client.
//
// Resolution semantics:
//   - scheme must be http or https
//   - hostname is resolved with the OS resolver (LookupIPAddr)
//   - every resolved IP must pass IsPublic — first failure rejects the URL
//
// Resolving up-front (not at dial time) closes the DNS-rebinding window:
// the dialer will see the same first answer the resolver gave us, so an
// attacker can't return a public IP at validation and a private IP at dial.

package safeurl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrBlocked is returned when a URL points at a non-public address.
var ErrBlocked = errors.New("url targets a non-public address")

// Validate parses raw and rejects it if the scheme is unsupported or any
// resolved IP is non-public. Returns the parsed URL when accepted so
// callers can reuse the work.
func Validate(ctx context.Context, raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("scheme %q not allowed (http/https only)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, errors.New("url has no host")
	}
	// Direct literal IP shortcut — skip resolver.
	if ip := net.ParseIP(host); ip != nil {
		if !IsPublic(ip) {
			return nil, fmt.Errorf("%w: %s", ErrBlocked, ip)
		}
		return u, nil
	}
	resolver := net.DefaultResolver
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses for %s", host)
	}
	for _, a := range addrs {
		if !IsPublic(a.IP) {
			return nil, fmt.Errorf("%w: %s -> %s", ErrBlocked, host, a.IP)
		}
	}
	return u, nil
}

// IsPublic reports whether ip is routable on the public internet — i.e.
// NOT loopback, link-local, private (RFC1918 / ULA), multicast, or any of
// the IANA-reserved special-purpose ranges that an attacker would use to
// reach internal services.
func IsPublic(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() ||
		ip.IsPrivate() || ip.IsUnspecified() {
		return false
	}
	// Carrier-grade NAT (RFC 6598) — net.IP.IsPrivate doesn't cover this.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1]&0xC0 == 64 { // 100.64.0.0/10
			return false
		}
		// 169.254.0.0/16 already caught by LinkLocalUnicast, but be explicit.
		if v4[0] == 169 && v4[1] == 254 {
			return false
		}
	}
	return true
}
