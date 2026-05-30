// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — MITM proxy capture/translator (TLS interception).

// MITM Paths (per-OS data dirs).

package mitm

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

const appName = "flow_router"

// DefaultDataDir returns the canonical per-OS data dir.
func DefaultDataDir() string {
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, _ := os.UserHomeDir()
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, appName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "."+appName)
}

// DataDir returns the effective data dir, honoring DATA_DIR / FLOW_ROUTER_DATA
// env (with writability fallback).
func DataDir() string {
	for _, k := range []string{"FLOW_ROUTER_DATA", "DATA_DIR"} {
		v := os.Getenv(k)
		if v == "" {
			continue
		}
		err := os.MkdirAll(v, 0o700)
		if err == nil {
			return v
		}
		if pe, ok := err.(*os.PathError); ok && pe.Err == os.ErrPermission {
			log.Printf("[DATA_DIR] %q not writable, falling back to default", v)
		}
	}
	return DefaultDataDir()
}

// MITMDir is <DataDir>/mitm.
func MITMDir() string { return filepath.Join(DataDir(), "mitm") }
