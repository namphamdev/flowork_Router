// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Platform-specific build constraint. No cross-platform risk.

//go:build windows

package updater

import (
	"os"
	"os/exec"
)

// restartImpl spawns a new process and exits the current one (Windows has no
// exec() that replaces the parent in-place; this is the standard substitute).
func restartImpl(exe string) error {
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
