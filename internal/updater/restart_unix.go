// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Platform-specific build constraint. No cross-platform risk.

//go:build !windows

package updater

import (
	"os"
	"syscall"
)

// restartImpl replaces the running process with a fresh exec.
func restartImpl(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
