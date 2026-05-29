// Package mesh — blocklist.go: cloud metadata IP denylist (INVARIANT 2).
//
// Per ANTI_KIAMAT_PROTOCOL.md §INVARIANT 2:
//   "Sistem mDNS atau pencarian lokal dilarang keras menerima respon route
//    yang mengarah ke IP Cloud Metadata (seperti 169.254.169.254)."
//
// Per M02-mesh-discovery-mdns.md: hardcoded denylist prevents Cloud Metadata
// Pivot Attacks by blocking link-local and known cloud metadata IPs/hostnames.
package mesh

import "net"

// cloudMetadataIPs — known cloud provider metadata service IPs.
var cloudMetadataIPs = map[string]bool{
	"169.254.169.254": true, // AWS, GCP, Azure
	"100.100.100.200": true, // Alibaba Cloud
	"192.0.0.192":     true, // legacy / reserved
}

// cloudMetadataDNS — known metadata hostnames (DNS spoofing defense).
var cloudMetadataDNS = map[string]bool{
	"metadata.google.internal":     true,
	"instance-data.ec2.internal":   true,
	"metadata.azure.com":           true,
	"metadata.internal":            true,
}

// linkLocalCIDR — entire 169.254.0.0/16 range (paranoid block).
var linkLocalCIDR *net.IPNet

func init() {
	_, linkLocalCIDR, _ = net.ParseCIDR("169.254.0.0/16")
}

// IsCloudMetadataIP returns true if IP matches known cloud metadata endpoints
// or falls within the link-local range. Used by mDNS discovery (INVARIANT 2)
// and gossip push handler.
func IsCloudMetadataIP(ip string) bool {
	if cloudMetadataIPs[ip] {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	// Block entire link-local range (paranoid — better safe than pivot attack)
	if linkLocalCIDR != nil && linkLocalCIDR.Contains(parsed) {
		return true
	}
	return false
}

// IsCloudMetadataHost returns true if hostname matches known cloud metadata DNS.
func IsCloudMetadataHost(host string) bool {
	return cloudMetadataDNS[host]
}
