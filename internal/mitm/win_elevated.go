// Windows elevation helper.

package mitm

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// IsAdmin reports whether the current process has admin/root privileges.
// Windows: runs `whoami /groups` and looks for the S-1-16-12288 (High Mandatory)
// SID. Unix: euid==0.
func IsAdmin() bool {
	if runtime.GOOS != "windows" {
		return os.Geteuid() == 0
	}
	out, err := exec.Command("whoami", "/groups").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "S-1-16-12288")
}

// RunElevatedPowerShell launches a PowerShell command via runas verb so a UAC
// prompt is shown. The actual command runs in a new elevated PowerShell.
// Returns nil when the elevation prompt was accepted (does NOT wait for command
// completion — same semantics as upstream's helper).
func RunElevatedPowerShell(command string) error {
	if runtime.GOOS != "windows" {
		return os.ErrInvalid
	}
	// Use PowerShell Start-Process -Verb RunAs to invoke ourselves elevated.
	starter := `Start-Process powershell -Verb RunAs -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-Command','` +
		strings.ReplaceAll(command, "'", "''") + `'`
	return exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", starter).Run()
}
