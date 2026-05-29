// Package mesh — discovery.go: mDNS LAN peer discovery (M2).
//
// Per M02-mesh-discovery-mdns.md:
// - mDNS multicast broadcast — peer di subnet sama auto-discover < 5 detik.
// - Service: _flowork._tcp on port 3105
// - TXT record: pubkey, version, virt flag
// - Cloud metadata IP blocked (INVARIANT 2 via blocklist.go)
//
// Implementation approach: pure Go UDP multicast (no CGO dependency).
// Uses standard mDNS/DNS-SD protocol on port 5353.
// If github.com/grandcat/zeroconf is added later, swap implementation.
//
// For now: broadcast-based discovery using simple UDP announce/listen
// pattern that works cross-OS without CGO.
package mesh

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

const (
	// mDNS multicast group + port (standard)
	mdnsIPv4Group = "224.0.0.251"
	mdnsPort      = 5353

	// Flowork-specific service identifier
	floworkServiceTag = "_flowork._tcp"

	// Announce interval — broadcast presence every 30 seconds
	announceInterval = 30 * time.Second

	// Discovery timeout per scan cycle
	discoveryTimeout = 5 * time.Second
)

// AnnouncePacket — broadcast by kernel to announce presence on LAN.
type AnnouncePacket struct {
	Service   string `json:"service"`     // "_flowork._tcp"
	PubKeyHex string `json:"pubkey"`      // ed25519 public key hex
	Port      int    `json:"port"`        // kernel port (3105)
	Version   string `json:"version"`     // kernel version
	IsVirt    bool   `json:"virt"`        // VM detection flag
	Hostname  string `json:"hostname"`    // OS hostname
	TS        int64  `json:"ts"`          // announce timestamp
}

// DiscoveredPeer — parsed from incoming announce packet.
type DiscoveredPeer struct {
	PubKeyHex     string
	IP            string
	Port          int
	Version       string
	IsVirtualized bool
	Hostname      string
	DiscoveredAt  time.Time
}

// Discovery — manages mDNS LAN peer discovery.
//
// Sprint 3.5e (BUG-W9 fix): WhitelistCheck optional callback untuk validate
// peer pubkey sebelum invoke onPeer. Default nil = accept all (backward compat
// untuk dev env). Production: caller wajib wire ke karma engine atau peer
// registry untuk reject unknown peers.
type Discovery struct {
	mu             sync.Mutex
	pubkey         []byte
	port           int
	version        string
	isVirt         bool
	onPeer         func(DiscoveredPeer)
	WhitelistCheck func(pubkeyHex string) bool // BUG-W9: optional whitelist filter
	conn           *net.UDPConn
	stopped        bool
}

// NewDiscovery creates a discovery service.
func NewDiscovery(pubkey []byte, port int, version string, isVirt bool, onPeer func(DiscoveredPeer)) *Discovery {
	return &Discovery{
		pubkey:  pubkey,
		port:    port,
		version: version,
		isVirt:  isVirt,
		onPeer:  onPeer,
	}
}

// Start begins both announcing and listening for peers.
// Blocks until ctx is cancelled.
func (d *Discovery) Start(ctx context.Context) error {
	// Resolve multicast address
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", mdnsIPv4Group, mdnsPort))
	if err != nil {
		return fmt.Errorf("discovery: resolve multicast: %w", err)
	}

	// Bind listener on all interfaces, mDNS port
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		// mDNS port might be in use — try ephemeral port for announce-only mode
		log.Printf("[discovery] mDNS port %d busy, running announce-only on ephemeral port: %v", mdnsPort, err)
		localAddr, _ := net.ResolveUDPAddr("udp4", ":0")
		conn, err = net.ListenUDP("udp4", localAddr)
		if err != nil {
			return fmt.Errorf("discovery: listen UDP: %w", err)
		}
	}
	d.conn = conn

	// Set read buffer
	_ = conn.SetReadBuffer(65536)

	log.Printf("[discovery] started on %s (pubkey=%s, port=%d, virt=%v)",
		conn.LocalAddr(), hex.EncodeToString(d.pubkey)[:8], d.port, d.isVirt)

	// Start announcer goroutine
	go d.announceLoop(ctx, addr)

	// Listen for incoming announcements
	return d.listenLoop(ctx)
}

// TriggerAnnounce sends an immediate announce (for POST /v1/mesh/discover).
func (d *Discovery) TriggerAnnounce() {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", mdnsIPv4Group, mdnsPort))
	if err != nil {
		return
	}
	d.announce(addr)
}

// Stop gracefully shuts down discovery.
func (d *Discovery) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopped = true
	if d.conn != nil {
		_ = d.conn.Close()
	}
}

// --- internal ---

func (d *Discovery) announceLoop(ctx context.Context, addr *net.UDPAddr) {
	// Immediate first announce
	d.announce(addr)

	ticker := time.NewTicker(announceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.announce(addr)
		}
	}
}

func (d *Discovery) announce(addr *net.UDPAddr) {
	d.mu.Lock()
	if d.stopped || d.conn == nil {
		d.mu.Unlock()
		return
	}
	d.mu.Unlock()

	hostname, _ := getHostname()
	pkt := AnnouncePacket{
		Service:   floworkServiceTag,
		PubKeyHex: hex.EncodeToString(d.pubkey),
		Port:      d.port,
		Version:   d.version,
		IsVirt:    d.isVirt,
		Hostname:  hostname,
		TS:        time.Now().Unix(),
	}

	data, err := json.Marshal(pkt)
	if err != nil {
		return
	}

	// Send to multicast group
	sendConn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		log.Printf("[discovery] announce dial fail: %v", err)
		return
	}
	defer sendConn.Close()

	if _, err := sendConn.Write(data); err != nil {
		log.Printf("[discovery] announce write fail: %v", err)
	}
}

func (d *Discovery) listenLoop(ctx context.Context) error {
	buf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Set read deadline to avoid blocking forever
		_ = d.conn.SetReadDeadline(time.Now().Add(2 * time.Second))

		n, remoteAddr, err := d.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // timeout is normal, loop again
			}
			d.mu.Lock()
			stopped := d.stopped
			d.mu.Unlock()
			if stopped {
				return nil
			}
			continue
		}

		// Parse announce packet
		var pkt AnnouncePacket
		if err := json.Unmarshal(buf[:n], &pkt); err != nil {
			continue // not a flowork packet, ignore
		}

		// Validate it's a flowork service
		if pkt.Service != floworkServiceTag {
			continue
		}

		// Skip self
		if pkt.PubKeyHex == hex.EncodeToString(d.pubkey) {
			continue
		}

		// INVARIANT 2: block cloud metadata IPs
		peerIP := remoteAddr.IP.String()
		if IsCloudMetadataIP(peerIP) {
			log.Printf("[discovery] REJECT: cloud metadata IP %s from peer %s (INVARIANT 2)",
				peerIP, pkt.PubKeyHex[:8])
			continue
		}

		// Valid peer discovered
		peer := DiscoveredPeer{
			PubKeyHex:     pkt.PubKeyHex,
			IP:            peerIP,
			Port:          pkt.Port,
			Version:       pkt.Version,
			IsVirtualized: pkt.IsVirt,
			Hostname:      pkt.Hostname,
			DiscoveredAt:  time.Now(),
		}

		log.Printf("[discovery] peer found: %s@%s:%d (virt=%v)",
			pkt.PubKeyHex[:8], peerIP, pkt.Port, pkt.IsVirt)

		// BUG-W9 fix Sprint 3.5e: whitelist filter sebelum invoke onPeer.
		// Default WhitelistCheck nil = accept all (backward compat). Caller
		// wire ke karma engine / peer registry untuk reject unknown peers.
		if d.WhitelistCheck != nil && !d.WhitelistCheck(pkt.PubKeyHex) {
			log.Printf("[discovery] peer %s REJECTED by whitelist", pkt.PubKeyHex[:8])
			continue
		}

		if d.onPeer != nil {
			d.onPeer(peer)
		}
	}
}

func getHostname() (string, error) {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown", nil
	}
	return h, nil
}
