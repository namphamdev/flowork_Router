// Package mesh — fingerprint.go: machine fingerprint cross-OS (M1).
//
// Per AMENDMENTS-V1 W-1: threshold-based 3-of-5 component match (bukan
// strict all-match). User ganti SSD / NIC USB / hostname rename ngga bikin
// license invalid. Only ≥3 component berubah → fingerprint mismatch hard.
//
// Components hashed (5 max):
//   1. Primary MAC address (most stable, but USB NIC swap = changed)
//   2. Hostname (stable kecuali rename)
//   3. OS type + arch (very stable)
//   4. CPU count (stable kecuali hardware swap)
//   5. User home dir path (stable per OS install)
//
// VM detection: Linux /sys/class/dmi/id/sys_vendor + /proc/version (WSL),
// Windows WMI manufacturer (TODO future). VM peer dapat karma 0.3 awal
// (vs 0.5 normal) per M5 karma engine.
//
// Cross-OS matrix:
//   - Windows: hostname + MAC + OS + arch + CPU count + USERPROFILE path
//   - Linux:   /etc/machine-id (kalau ada) + MAC + OS + arch + CPU count + HOME
//   - macOS:   ioreg fallback ke MAC + hostname (no DMI)
//
// Fingerprint = hex(sha256(component_1 || \x00 || ... || component_n)).
// Component count selalu 5 (zero-fill kalau ngga ke-detect).

package mesh

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// FingerprintComponent name+value untuk threshold matching.
type FingerprintComponent struct {
	Name  string
	Value string
}

// MachineFingerprint return SHA-256 hex dari 5 components, fixed order.
//
// Untuk threshold match (W-1), pakai MachineFingerprintComponents() yang
// return list of 5 individual hashes — caller bisa compare per-component.
func MachineFingerprint() string {
	comps := MachineFingerprintComponents()
	var parts []string
	for _, c := range comps {
		parts = append(parts, c.Value)
	}
	joined := strings.Join(parts, "\x00")
	h := sha256.Sum256([]byte(joined))
	return "sha256:" + hex.EncodeToString(h[:])
}

// MachineFingerprintComponents return 5 components (always 5, zero-fill missing).
//
// Order STABIL: index 0=MAC, 1=hostname, 2=OS+arch, 3=CPU count, 4=home dir.
// Threshold match (W-1): kalau ≥3 component match prior fingerprint =
// device sama (allow grace). Kalau <3 = device beda (reject).
func MachineFingerprintComponents() []FingerprintComponent {
	out := make([]FingerprintComponent, 5)

	// 0. Primary MAC
	if mac := primaryMAC(); mac != "" {
		out[0] = FingerprintComponent{Name: "mac", Value: hashShort("mac:" + mac)}
	} else {
		out[0] = FingerprintComponent{Name: "mac", Value: "missing"}
	}

	// 1. Hostname
	if h, err := os.Hostname(); err == nil && h != "" {
		out[1] = FingerprintComponent{Name: "hostname", Value: hashShort("host:" + h)}
	} else {
		out[1] = FingerprintComponent{Name: "hostname", Value: "missing"}
	}

	// 2. OS + arch (very stable per device)
	out[2] = FingerprintComponent{
		Name:  "os_arch",
		Value: hashShort("os:" + runtime.GOOS + ":" + runtime.GOARCH),
	}

	// 3. CPU count
	out[3] = FingerprintComponent{
		Name:  "cpu_count",
		Value: hashShort("ncpu:" + strconv.Itoa(runtime.NumCPU())),
	}

	// 4. User home dir path
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		out[4] = FingerprintComponent{Name: "home_dir", Value: hashShort("home:" + home)}
	} else {
		out[4] = FingerprintComponent{Name: "home_dir", Value: "missing"}
	}

	return out
}

// FingerprintThresholdMatch return true kalau ≥minMatch components sama
// antara prior dan current (W-1 threshold-based matching).
//
// Default minMatch = 3 (3-of-5 = grace untuk hardware ganti tunggal).
func FingerprintThresholdMatch(prior, current []FingerprintComponent, minMatch int) bool {
	if minMatch <= 0 {
		minMatch = 3
	}
	if len(prior) != len(current) {
		return false
	}
	matches := 0
	for i := range prior {
		if prior[i].Value == current[i].Value && prior[i].Value != "missing" {
			matches++
		}
	}
	return matches >= minMatch
}

// IsVirtualizedHost detect VM/container signature. Best-effort.
//
// VM peer di mesh dapat karma awal 0.3 (vs 0.5 normal) per M5.
func IsVirtualizedHost() bool {
	switch runtime.GOOS {
	case "linux":
		// /sys/class/dmi/id/sys_vendor — VirtualBox, VMware, QEMU, KVM, Microsoft Corporation
		if data, err := os.ReadFile("/sys/class/dmi/id/sys_vendor"); err == nil {
			s := strings.ToLower(strings.TrimSpace(string(data)))
			for _, marker := range []string{"vmware", "virtualbox", "qemu", "xen", "kvm", "microsoft corporation", "innotek"} {
				if strings.Contains(s, marker) {
					return true
				}
			}
		}
		// /proc/version — WSL detection
		if data, err := os.ReadFile("/proc/version"); err == nil {
			s := strings.ToLower(string(data))
			if strings.Contains(s, "microsoft") || strings.Contains(s, "wsl") {
				return true
			}
		}
		// Container detection — /.dockerenv exists
		if _, err := os.Stat("/.dockerenv"); err == nil {
			return true
		}
		// LXC/LXD detection
		if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
			if strings.Contains(string(data), "/docker/") || strings.Contains(string(data), "/lxc/") {
				return true
			}
		}
	case "windows":
		// Heuristic: SystemManufacturer environment + virtualization-only paths
		// Full WMI detection deferred to future (no CGO needed but adds dep).
		// Conservative: skip detection on Windows for now.
	case "darwin":
		// VM less common on macOS dev. Future: ioreg parse.
	}
	return false
}

// --- helpers ---

func primaryMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 {
			continue
		}
		mac := i.HardwareAddr.String()
		if mac != "" && mac != "00:00:00:00:00:00" {
			return mac
		}
	}
	return ""
}

func hashShort(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8]) // 16 hex chars (8 bytes) = enough for component fingerprint
}
