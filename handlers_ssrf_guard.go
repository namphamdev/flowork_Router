// SSRF guard for operator-configured outbound URLs (MCP HTTP gateway, media
// provider base URLs). Unlike internal/safeurl (which the /v1/web/fetch proxy
// uses to block ALL private space), this guard intentionally ALLOWS loopback
// and private/LAN addresses — running a local MCP or media server on
// 127.0.0.1 / 192.168.x is a legitimate use case for this single-user tool.
// It blocks ONLY link-local space (169.254.0.0/16 + fe80::/10), which is the
// cloud-metadata credential-exfiltration vector (169.254.169.254, fd00:ec2::254
// is link-local-mapped via IMDS). That cuts the dangerous SSRF target without
// breaking local-server functionality.

package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// blockMetadataURL resolves raw and rejects it if any resolved IP is link-local
// (cloud-metadata range). Empty input returns ("", nil) so callers that already
// special-case an empty URL keep their existing behaviour. On success it returns
// the normalized URL string.
func blockMetadataURL(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("scheme %q not allowed (http/https only)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("empty host")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", host, err)
	}
	for _, ip := range ips {
		ip = ip.Unmap()
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return "", fmt.Errorf("blocked link-local/metadata target %s (SSRF guard)", ip)
		}
	}
	return u.String(), nil
}
