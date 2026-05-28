// cmdTray launches the per-OS tray helper script. On Windows, that is a
// PowerShell script using System.Windows.Forms.NotifyIcon (real native tray);
// on Linux/macOS it is a shell script providing a CGO-free control surface
// (notify-send / osascript + xdg-open / open).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func cmdTray(args []string) error {
	scripts := locateScriptsDir()
	switch runtime.GOOS {
	case "windows":
		ps := filepath.Join(scripts, "tray-win.ps1")
		if _, err := os.Stat(ps); err != nil {
			return fmt.Errorf("tray-win.ps1 not found at %s", ps)
		}
		c := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", ps)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Start()
	case "darwin":
		sh := filepath.Join(scripts, "tray-mac.sh")
		return runShell(sh, args)
	case "linux":
		sh := filepath.Join(scripts, "tray-linux.sh")
		return runShell(sh, args)
	default:
		return fmt.Errorf("tray not supported on %s yet", runtime.GOOS)
	}
}

func runShell(script string, args []string) error {
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("script not found: %s", script)
	}
	all := append([]string{script}, args...)
	c := exec.Command("bash", all...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// locateScriptsDir returns the path to the bundled scripts/. Tries (in order):
// FLOW_ROUTER_SCRIPTS env, ./scripts (CWD), and ../scripts (binary's parent).
func locateScriptsDir() string {
	if d := os.Getenv("FLOW_ROUTER_SCRIPTS"); d != "" {
		return d
	}
	if cwd, err := os.Getwd(); err == nil {
		p := filepath.Join(cwd, "scripts")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "..", "scripts")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return "./scripts"
}
