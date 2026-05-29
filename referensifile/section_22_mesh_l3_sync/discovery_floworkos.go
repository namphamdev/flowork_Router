// Package mesh implements peer-to-peer networking for FLOWORK agents.
// Discovery (this file) uses mDNS (RFC 6762) so agents on the same LAN
// find each other automatically — no central server, no manual config.
package mesh

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	// ServiceType registered with mDNS. Other flowork agents browse this
	// exact string to find us.
	ServiceType = "_flowork-mesh._tcp"
	Domain      = "local."
)

// Peer describes another flowork agent discovered on the LAN.
type Peer struct {
	ID       string    // InstanceName (hostname-<rand>)
	Addrs    []net.IP  // IPv4/IPv6 addresses from mDNS A/AAAA records
	Port     int       // TCP port where peer's transport server listens
	PubKey   string    // hex-encoded NaCl box public key (from TXT record)
	LastSeen time.Time // updated each time the peer is re-announced
}

// Discovery advertises this agent on mDNS and tracks peers.
// Zero-value is NOT valid — use NewDiscovery.
type Discovery struct {
	instanceID string
	port       int
	pubKeyHex  string

	server *zeroconf.Server

	mu    sync.RWMutex
	peers map[string]*Peer // keyed by InstanceName
}

// NewDiscovery creates a discovery service that will announce this agent
// as listening on the given TCP port with the given NaCl box public key.
// The instanceID should be stable across restarts for reconnection —
// hostname + short hash works well.
func NewDiscovery(instanceID string, port int, pubKeyHex string) *Discovery {
	if instanceID == "" {
		host, _ := os.Hostname()
		instanceID = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	return &Discovery{
		instanceID: instanceID,
		port:       port,
		pubKeyHex:  pubKeyHex,
		peers:      make(map[string]*Peer),
	}
}

// Start advertises this agent and begins browsing for peers.
// Blocks until ctx is cancelled. Returns when ctx done or registration fails.
func (d *Discovery) Start(ctx context.Context) error {
	txt := []string{
		"flowork=1",
		"pubkey=" + d.pubKeyHex,
	}
	srv, err := zeroconf.Register(d.instanceID, ServiceType, Domain, d.port, txt, nil)
	if err != nil {
		return fmt.Errorf("mdns register: %w", err)
	}
	d.server = srv
	defer srv.Shutdown()

	// Browse for other peers in a goroutine. Re-browse every 30s so peers
	// that join later are discovered promptly.
	browseCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go d.browseLoop(browseCtx)

	<-ctx.Done()
	return nil
}

// browseLoop continuously browses for mDNS peers and feeds them to ingest.
func (d *Discovery) browseLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	d.browseOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.EvictOldPeers()
			d.browseOnce(ctx)
		}
	}
}

// EvictOldPeers menghapus rekan lama dari memori untuk mencegah Memory Leak.
func (d *Discovery) EvictOldPeers() {
	d.mu.Lock()
	defer d.mu.Unlock()
	cutoff := time.Now().Add(-5 * time.Minute)
	for id, p := range d.peers {
		if p.LastSeen.Before(cutoff) {
			delete(d.peers, id)
		}
	}
}

// browseOnce runs a single 10-second mDNS browse. Each hit is funneled
// through ingest which dedups on InstanceName.
func (d *Discovery) browseOnce(ctx context.Context) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return
	}
	entries := make(chan *zeroconf.ServiceEntry, 16)

	browseCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	go func() {
		for e := range entries {
			if e.Instance == d.instanceID {
				continue // ignore ourselves
			}
			d.ingest(e)
		}
	}()

	_ = resolver.Browse(browseCtx, ServiceType, Domain, entries)
	<-browseCtx.Done()
}

// ingest records a discovered peer or refreshes its LastSeen timestamp.
func (d *Discovery) ingest(e *zeroconf.ServiceEntry) {
	addrs := make([]net.IP, 0, len(e.AddrIPv4)+len(e.AddrIPv6))
	addrs = append(addrs, e.AddrIPv4...)
	addrs = append(addrs, e.AddrIPv6...)
	if len(addrs) == 0 {
		return
	}

	pubKey := ""
	for _, t := range e.Text {
		if len(t) > 7 && t[:7] == "pubkey=" {
			pubKey = t[7:]
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	existing, ok := d.peers[e.Instance]
	if !ok {
		d.peers[e.Instance] = &Peer{
			ID:       e.Instance,
			Addrs:    addrs,
			Port:     e.Port,
			PubKey:   pubKey,
			LastSeen: time.Now(),
		}
		return
	}
	existing.Addrs = addrs
	existing.Port = e.Port
	existing.PubKey = pubKey
	existing.LastSeen = time.Now()
}

// Peers returns a snapshot of currently-known peers.
// Peers not seen for >5 minutes are filtered out.
func (d *Discovery) Peers() []Peer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	out := make([]Peer, 0, len(d.peers))
	for _, p := range d.peers {
		if p.LastSeen.Before(cutoff) {
			continue
		}
		out = append(out, *p)
	}
	return out
}

// Stop tears down the mDNS advertisement.
func (d *Discovery) Stop() {
	if d.server != nil {
		d.server.Shutdown()
	}
}
