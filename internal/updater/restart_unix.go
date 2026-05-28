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
