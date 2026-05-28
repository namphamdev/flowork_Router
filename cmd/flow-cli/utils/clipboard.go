// Clipboard helper: best-effort multi-OS copy via the OS native tool.
package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Copy writes s to the system clipboard. Returns nil on success, otherwise a
// human-readable error suitable for display.
func Copy(s string) error {
	switch runtime.GOOS {
	case "darwin":
		return run("pbcopy", s)
	case "windows":
		return run("clip", s)
	case "linux":
		// Try wl-copy (Wayland) then xclip (X11) then xsel.
		for _, tool := range [][]string{
			{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"},
		} {
			if _, err := exec.LookPath(tool[0]); err == nil {
				return run(tool[0], s, tool[1:]...)
			}
		}
		return fmt.Errorf("no clipboard tool found (install wl-copy, xclip, or xsel)")
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
}

func run(name, payload string, args ...string) error {
	c := exec.Command(name, args...)
	stdin, err := c.StdinPipe()
	if err != nil {
		return err
	}
	if err := c.Start(); err != nil {
		return err
	}
	_, _ = strings.NewReader(payload).WriteTo(stdin)
	stdin.Close()
	return c.Wait()
}
