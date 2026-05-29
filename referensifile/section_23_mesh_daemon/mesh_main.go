// flowork-mesh — peer-to-peer mesh daemon.
//
// Wires together discovery (mDNS), transport (TCP+NaCl), and sync (hash-chain
// log) so multiple FLOWORK agents on the same LAN keep their shared inbox
// in sync without internet, central server, or manual config.
//
// Subcommands:
//
//	flowork-mesh start                 # run as daemon (advertise + sync)
//	flowork-mesh status                # print known peers + log tip
//	flowork-mesh join <peer-pubkey>    # whitelist a peer's pubkey
//
// On first run, generates a NaCl keypair at ~/.flowork/mesh_key and a peer
// allowlist at ~/.flowork/mesh_peers.json. Both files are mode 0600.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/teetah2402/flowork/internal/mesh"
)

const version = "0.1.0"

type keyFile struct {
	PublicHex  string `json:"public"`
	PrivateHex string `json:"private"`
}

type peersFile struct {
	Allowed []string `json:"allowed_pubkeys"` // hex pubkeys
}

func meshDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".flowork")
}

func keyPath() string  { return filepath.Join(meshDir(), "mesh_key") }
func peerPath() string { return filepath.Join(meshDir(), "mesh_peers.json") }
func logPath() string  { return filepath.Join(meshDir(), "mesh_log.jsonl") }

// loadOrCreateKeys reads the persisted keypair or generates one.
func loadOrCreateKeys() (*mesh.KeyPair, error) {
	data, err := os.ReadFile(keyPath())
	if err == nil {
		var kf keyFile
		if err := json.Unmarshal(data, &kf); err == nil && len(kf.PublicHex) == 64 && len(kf.PrivateHex) == 64 {
			pubBytes, _ := hex.DecodeString(kf.PublicHex)
			privBytes, _ := hex.DecodeString(kf.PrivateHex)
			var pub, priv [32]byte
			copy(pub[:], pubBytes)
			copy(priv[:], privBytes)
			return &mesh.KeyPair{Public: &pub, Private: &priv}, nil
		}
	}

	kp, err := mesh.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	kf := keyFile{
		PublicHex:  hex.EncodeToString(kp.Public[:]),
		PrivateHex: hex.EncodeToString(kp.Private[:]),
	}
	if err := os.MkdirAll(meshDir(), 0o700); err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(kf, "", "  ")
	if err := os.WriteFile(keyPath(), b, 0o600); err != nil {
		return nil, err
	}
	return kp, nil
}

func loadAllowedPeers() []string {
	data, err := os.ReadFile(peerPath())
	if err != nil {
		return nil
	}
	var pf peersFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil
	}
	return pf.Allowed
}

func saveAllowedPeers(pubs []string) error {
	pf := peersFile{Allowed: pubs}
	b, _ := json.MarshalIndent(pf, "", "  ")
	if err := os.MkdirAll(meshDir(), 0o700); err != nil {
		return fmt.Errorf("main: saveAllowedPeers: mkdir: %w", err)
	}
	return os.WriteFile(peerPath(), b, 0o600)
}

func cmdStatus() error {
	kp, err := loadOrCreateKeys()
	if err != nil {
		return fmt.Errorf("load keys: %w", err)
	}
	fmt.Println("flowork-mesh", version)
	fmt.Println("our pubkey  :", kp.PublicHex())
	fmt.Println("key file    :", keyPath())
	fmt.Println("peer file   :", peerPath())
	fmt.Println("log file    :", logPath())
	allowed := loadAllowedPeers()
	fmt.Printf("allowed peers (%d):\n", len(allowed))
	for _, p := range allowed {
		fmt.Println("  -", p)
	}
	if log, err := mesh.NewSyncLog(logPath()); err == nil {
		fmt.Println("log entries :", log.Len())
		fmt.Println("log tip     :", log.Tip())
	}
	return nil
}

func cmdJoin(pubHex string) error {
	pubHex = strings.TrimSpace(pubHex)
	if len(pubHex) != 64 {
		return fmt.Errorf("pubkey must be 64 hex chars, got %d", len(pubHex))
	}
	if _, err := hex.DecodeString(pubHex); err != nil {
		return fmt.Errorf("invalid hex: %w", err)
	}
	allowed := loadAllowedPeers()
	for _, p := range allowed {
		if p == pubHex {
			fmt.Println("peer already allowed:", pubHex)
			return nil
		}
	}
	allowed = append(allowed, pubHex)
	if err := saveAllowedPeers(allowed); err != nil {
		return fmt.Errorf("main: cmdJoin: %w", err)
	}
	fmt.Println("added peer:", pubHex)
	return nil
}

func cmdStart(port int) error {
	kp, err := loadOrCreateKeys()
	if err != nil {
		return fmt.Errorf("load keys: %w", err)
	}
	log, err := mesh.NewSyncLog(logPath())
	if err != nil {
		return fmt.Errorf("open sync log: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; cancel() }()

	// Inbound message handler: merge into the sync log.
	handler := func(peerPub *[32]byte, msg []byte) ([]byte, error) {
		var entries []mesh.HashChainEntry
		if err := json.Unmarshal(msg, &entries); err != nil {
			return nil, fmt.Errorf("decode msg: %w", err)
		}
		merged, err := log.MergeEntries(entries)
		if err != nil {
			return nil, fmt.Errorf("merge: %w", err)
		}
		ack := fmt.Sprintf(`{"merged":%d,"tip":%q}`, merged, log.Tip())
		return []byte(ack), nil
	}

	listener, err := mesh.NewListener(port, true, kp, handler)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	for _, p := range loadAllowedPeers() {
		raw, _ := hex.DecodeString(p)
		var pub [32]byte
		copy(pub[:], raw)
		listener.AllowPeer(&pub)
	}

	disc := mesh.NewDiscovery("", listener.Port(), kp.PublicHex())

	fmt.Fprintln(os.Stderr, "flowork-mesh", version, "started")
	fmt.Fprintln(os.Stderr, "  pubkey :", kp.PublicHex())
	fmt.Fprintln(os.Stderr, "  port   :", listener.Port())
	fmt.Fprintln(os.Stderr, "  log    :", logPath(), "(", log.Len(), "entries )")
	fmt.Fprintln(os.Stderr, "  Press Ctrl+C to stop.")

	// M-14 (rc170): build fast lookup of allowed pubkeys untuk dipakai
	// outbound sync loop — filter peer discovery sebelum dial.
	allowedPubs := make(map[string]bool, len(loadAllowedPeers()))
	for _, p := range loadAllowedPeers() {
		allowedPubs[p] = true
	}

	go func() {
		if err := listener.Serve(ctx); err != nil && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "listener error:", err)
		}
	}()
	go reportPeersLoop(ctx, disc)
	// M-14 (rc170): outbound push-sync loop — dial tiap allowed peer
	// via V2 transport, push hash-chain entries yang lokal. Dedup di sisi
	// peer via MergeEntries HasHash. Interval 60s balance bandwidth vs
	// freshness.
	go peerSyncLoop(ctx, kp, disc, log, allowedPubs)

	if err := disc.Start(ctx); err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	return nil
}

// peerSyncLoop dials each known allowed peer every 60s via V2 transport
// and pushes local hash-chain entries. Peer's listener handler (line
// ~169) merges via MergeEntries yang dedup-by-hash. V2 = challenge-
// response handshake untuk replay defense (EXTBUG-007/7.1/7.2 coverage).
//
// Push-all strategy (MVP): send all in-memory entries every cycle.
// Cost: O(N entries × M peers × 60s). OK sampai N > 5000 atau M > 10.
// Optimasi future: track per-peer last-tip untuk incremental push.
func peerSyncLoop(ctx context.Context, kp *mesh.KeyPair, disc *mesh.Discovery, log *mesh.SyncLog, allowedPubs map[string]bool) {
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			pushToPeers(ctx, kp, disc, log, allowedPubs)
		}
	}
}

// pushToPeers iterates discovered peers, filters by allowlist, dials via
// V2 transport, sends local hash-chain entries as JSON payload. Skip
// silently kalau V2 transport tidak tersedia, peer tidak di allowlist,
// atau addr kosong. Error per-peer di-log ke stderr, tidak propagate —
// satu peer mati tidak boleh halt sync ke peer lain.
func pushToPeers(ctx context.Context, kp *mesh.KeyPair, disc *mesh.Discovery, log *mesh.SyncLog, allowedPubs map[string]bool) {
	v2 := mesh.GetTransport("v2")
	if v2 == nil {
		return
	}
	entries := log.EntriesSince("")
	if len(entries) == 0 {
		return
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[mesh sync] marshal: %v\n", err)
		return
	}
	for _, p := range disc.Peers() {
		if !allowedPubs[p.PubKey] || len(p.Addrs) == 0 {
			continue
		}
		pubBytes, err := hex.DecodeString(p.PubKey)
		if err != nil || len(pubBytes) != 32 {
			continue
		}
		var peerPub [32]byte
		copy(peerPub[:], pubBytes)
		addr := fmt.Sprintf("%s:%d", p.Addrs[0], p.Port)

		ctxDial, cancel := context.WithTimeout(ctx, 30*time.Second)
		sess, derr := v2.Dial(ctxDial, addr, kp, &peerPub)
		cancel()
		if derr != nil {
			fmt.Fprintf(os.Stderr, "[mesh sync] dial %s (%s): %v\n", p.ID, addr, derr)
			continue
		}
		if serr := sess.Send(payload); serr != nil {
			fmt.Fprintf(os.Stderr, "[mesh sync] send %s: %v\n", p.ID, serr)
			sess.Close()
			continue
		}
		if ack, rerr := sess.Recv(); rerr == nil {
			fmt.Fprintf(os.Stderr, "[mesh sync] ack %s: %s\n", p.ID, string(ack))
		}
		sess.Close()
	}
}

// reportPeersLoop logs newly-seen peers to stderr every 30s.
func reportPeersLoop(ctx context.Context, d *mesh.Discovery) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			peers := d.Peers()
			if len(peers) == 0 {
				continue
			}
			fmt.Fprintf(os.Stderr, "[mesh] %d peer(s) visible:\n", len(peers))
			for _, p := range peers {
				fmt.Fprintf(os.Stderr, "  - %s @ %v:%d  pubkey=%s...\n",
					p.ID, p.Addrs, p.Port, p.PubKey[:16])
			}
		}
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `flowork-mesh `+version+` — P2P mesh daemon

Usage:
  flowork-mesh start [--port N]      Run as daemon (advertise + sync)
  flowork-mesh status                Print own pubkey, peers, log tip
  flowork-mesh join <pubkey-hex>     Whitelist a peer's public key
  flowork-mesh --help                Show this message`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "start":
		port := 0 // OS-assigned by default
		for i := 2; i < len(os.Args); i++ {
			if os.Args[i] == "--port" && i+1 < len(os.Args) {
				if n, err := strconv.Atoi(os.Args[i+1]); err == nil {
					port = n
				}
				i++
			}
		}
		if err := cmdStart(port); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "status":
		if err := cmdStatus(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "join":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: flowork-mesh join <pubkey-hex>")
			os.Exit(1)
		}
		if err := cmdJoin(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "--help", "-h", "help":
		usage()
	case "--version", "-v":
		fmt.Println("flowork-mesh", version)
	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand:", os.Args[1])
		usage()
		os.Exit(1)
	}
}
