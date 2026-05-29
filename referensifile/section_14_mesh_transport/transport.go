// Package mesh — transport.go: M9 transport interface + USB sneakernet
// + Bluetooth/LoRa stubs (per AMENDMENTS-V1 I-7).
//
// Per ANTI_KIAMAT_PROTOCOL.md: mesh harus tetap operasional saat WAN down,
// pakai fallback transport (Bluetooth, LoRa, USB sneakernet).
//
// Implementations:
//   - mDNS LAN: discovery.go (M2, primary)
//   - USB sneakernet: this file (cross-OS via filesystem)
//   - Bluetooth: stub interface (Linux/Android future implementation)
//   - LoRa: stub interface (SX1276 module 433/915MHz, future)

package mesh

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// Transport common interface untuk semua fallback transport.
type Transport interface {
	Name() string
	// Push send batch ke target. target = peer-specific (file path untuk USB,
	// MAC untuk Bluetooth, frequency untuk LoRa).
	Push(target string, batch PushBatch) error
	// Receive blocking call yang return packets saat ada incoming. Caller
	// pass ctx untuk cancel.
	Receive(ctx context.Context) (<-chan PushBatch, error)
	// Close cleanup resources.
	Close() error
}

// ---------- USB Sneakernet ----------

// USBTransport sneakernet via filesystem. Cross-OS — works di Linux, Windows,
// macOS via standard mount points.
//
// Format file: <mount>/flowork-mesh/packets-<unix_nano>.json
type USBTransport struct {
	masterDir string // base dir untuk write+watch (default: detected USB mount)
	// BUG-110 fix 2026-05-01: closed flag accessed dari 2 goroutine berbeda
	// (Receive loop reads di line ~123, Close writes di line ~161). Plain
	// bool = data race per Go spec. atomic.Bool = lock-free safe.
	closed atomic.Bool
}

// NewUSBTransport construct dengan optional masterDir override (default
// auto-detect first removable mount). Caller passes "" untuk auto-detect.
func NewUSBTransport(masterDir string) *USBTransport {
	if masterDir == "" {
		masterDir = autoDetectUSBMount()
	}
	return &USBTransport{masterDir: masterDir}
}

// Name return identifier.
func (u *USBTransport) Name() string { return "usb_sneakernet" }

// Push write batch ke USB mount as JSON file.
//
// target = explicit mount path (override masterDir) atau "" untuk default.
func (u *USBTransport) Push(target string, batch PushBatch) error {
	if u.closed.Load() {
		return errors.New("transport closed")
	}
	dir := u.masterDir
	if target != "" {
		dir = target
	}
	if dir == "" {
		return errors.New("no USB mount configured")
	}

	subDir := filepath.Join(dir, "flowork-mesh")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		return fmt.Errorf("mkdir USB: %w", err)
	}

	fname := filepath.Join(subDir, fmt.Sprintf("packets-%d.json", time.Now().UnixNano()))
	data, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return err
	}
	tmp := fname + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, fname)
}

// Receive scan masterDir/flowork-mesh/ untuk file baru, parse, push ke channel.
//
// Polling-based (every 5s) untuk simplicity. Real prod akan pakai inotify/
// FSEvents/ReadDirectoryChangesW.
func (u *USBTransport) Receive(ctx context.Context) (<-chan PushBatch, error) {
	if u.masterDir == "" {
		return nil, errors.New("no USB mount configured")
	}
	out := make(chan PushBatch, 16)
	go u.receiveLoop(ctx, out)
	return out, nil
}

func (u *USBTransport) receiveLoop(ctx context.Context, out chan<- PushBatch) {
	defer close(out)
	seen := make(map[string]bool)
	subDir := filepath.Join(u.masterDir, "flowork-mesh")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if u.closed.Load() {
			return
		}
		entries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			fpath := filepath.Join(subDir, entry.Name())
			if seen[fpath] {
				continue
			}
			seen[fpath] = true
			data, err := os.ReadFile(fpath)
			if err != nil {
				continue
			}
			var batch PushBatch
			if err := json.Unmarshal(data, &batch); err != nil {
				continue
			}
			select {
			case out <- batch:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Close shutdown. BUG-110 fix: atomic.Store buat sync dengan Load di
// Push + Receive loop (concurrent-safe).
func (u *USBTransport) Close() error {
	u.closed.Store(true)
	return nil
}

// autoDetectUSBMount cross-OS heuristic. Best-effort.
func autoDetectUSBMount() string {
	// Linux: /media/<user>/, /run/media/<user>/, /mnt/
	// macOS: /Volumes/
	// Windows: scan drive letters D:\ - Z:\
	// MVP: skip (caller pass explicit path). Return "".
	return ""
}

// ---------- Bluetooth (stub — Linux/Android future) ----------

// BluetoothTransport stub. Real implementation di future PR pakai
// `tinygo.org/x/bluetooth` (Linux/Android only) atau WinRT (Windows).
type BluetoothTransport struct{}

// NewBluetoothTransport construct stub.
func NewBluetoothTransport() *BluetoothTransport { return &BluetoothTransport{} }

func (b *BluetoothTransport) Name() string { return "bluetooth_ble" }

// Push not implemented.
func (b *BluetoothTransport) Push(_ string, _ PushBatch) error {
	return errors.New("bluetooth transport: not implemented (Phase 2)")
}

// Receive not implemented.
func (b *BluetoothTransport) Receive(_ context.Context) (<-chan PushBatch, error) {
	return nil, errors.New("bluetooth transport: not implemented (Phase 2)")
}

// Close no-op.
func (b *BluetoothTransport) Close() error { return nil }

// ---------- LoRa (stub — SX1276 module future) ----------

// LoRaTransport stub. Per AMENDMENTS-V1 I-7:
//   - Module: SX1276 (10-15km urban, 20km+ open area)
//   - Bandwidth: 250bps-50kbps (cukup untuk drawer text, NOT V4 weight)
//   - Frequency: 433MHz (Indonesia umum) / 915MHz (cek SDPPI)
//   - Max 10 packet/jam (spectrum courtesy)
//   - ARQ untuk reliability
type LoRaTransport struct {
	SerialPort string
	BaudRate   int
	Frequency  int
	MaxPerHour int
}

// NewLoRaTransport construct stub.
func NewLoRaTransport(serialPort string, freq int) *LoRaTransport {
	return &LoRaTransport{
		SerialPort: serialPort,
		BaudRate:   115200,
		Frequency:  freq,
		MaxPerHour: 10,
	}
}

func (l *LoRaTransport) Name() string { return "lora_sx1276" }

func (l *LoRaTransport) Push(_ string, _ PushBatch) error {
	return errors.New("lora transport: not implemented (Phase 2 — needs SDPPI clearance Indonesia)")
}

func (l *LoRaTransport) Receive(_ context.Context) (<-chan PushBatch, error) {
	return nil, errors.New("lora transport: not implemented (Phase 2)")
}

func (l *LoRaTransport) Close() error { return nil }

// ---------- Doomsday Drill ----------

// DrillScenario one test step.
type DrillScenario struct {
	Name      string
	Transport string  // "usb_sneakernet" | "bluetooth_ble" | "lora_sx1276"
	WANState  string  // "off" | "on"
	Action    string  // "push_to_target" | "verify_received" | "tamper_payload"
	Args      map[string]string
}

// DrillReport final outcome.
type DrillReport struct {
	Name       string         `json:"name"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
	Results    []DrillResult  `json:"results"`
}

// DrillResult per-scenario outcome.
type DrillResult struct {
	Scenario string `json:"scenario"`
	Pass     bool   `json:"pass"`
	Reason   string `json:"reason"`
	Duration string `json:"duration"`
}

// DrillRunner execute scenario sequence + collect results.
type DrillRunner struct {
	Transports map[string]Transport
}

// NewDrillRunner construct dengan transport map (pre-init).
func NewDrillRunner(transports map[string]Transport) *DrillRunner {
	return &DrillRunner{Transports: transports}
}

// RunScenario execute single scenario. Return DrillResult pass/fail.
func (r *DrillRunner) RunScenario(ctx context.Context, s DrillScenario) DrillResult {
	start := time.Now()
	result := DrillResult{Scenario: s.Name}
	defer func() {
		result.Duration = time.Since(start).String()
	}()

	transport, ok := r.Transports[s.Transport]
	if !ok {
		result.Reason = "transport not registered: " + s.Transport
		return result
	}
	switch s.Action {
	case "push_to_target":
		// Build dummy batch
		batch := PushBatch{
			FromPubKey: []byte("drill-runner"),
			Timestamp:  time.Now().UnixNano(),
			Packets:    []KnowledgePacket{},
		}
		target := s.Args["target"]
		if err := transport.Push(target, batch); err != nil {
			result.Reason = "push failed: " + err.Error()
			return result
		}
		result.Pass = true
		result.Reason = "push success"
	case "verify_received":
		// Caller pass expected_min count
		minCount := 1
		if v, ok := s.Args["min_count"]; ok {
			fmt.Sscanf(v, "%d", &minCount)
		}
		ch, err := transport.Receive(ctx)
		if err != nil {
			result.Reason = "receive failed: " + err.Error()
			return result
		}
		received := 0
		timeout := time.After(10 * time.Second)
		for received < minCount {
			select {
			case <-timeout:
				result.Reason = fmt.Sprintf("timeout: received %d/%d", received, minCount)
				return result
			case <-ctx.Done():
				result.Reason = "context cancelled"
				return result
			case _, ok := <-ch:
				if !ok {
					result.Reason = fmt.Sprintf("channel closed early: %d/%d", received, minCount)
					return result
				}
				received++
			}
		}
		result.Pass = true
		result.Reason = fmt.Sprintf("received %d packets", received)
	default:
		result.Reason = "unknown action: " + s.Action
	}
	return result
}

// USBSignedBatch helper untuk USB sneakernet test — sign batch dengan
// trusted key supaya filter L1 di receiver lulus.
func USBSignedBatch(packets []KnowledgePacket, fromPub []byte, fromPriv ed25519.PrivateKey) PushBatch {
	batch := PushBatch{
		FromPubKey: fromPub,
		Timestamp:  time.Now().UnixNano(),
		Packets:    packets,
	}
	canonical := canonicalBatchBytes(batch)
	hash := sha256.Sum256(canonical)
	batch.Signature = ed25519.Sign(fromPriv, hash[:])
	return batch
}
