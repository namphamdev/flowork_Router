// MITM DNS hijack (hosts file edit, multi-OS).

package mitm

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const dnsMarker = "# flow_router MITM begin"
const dnsMarkerEnd = "# flow_router MITM end"

// HostsFilePath returns the per-OS hosts file.
func HostsFilePath() string {
	if runtime.GOOS == "windows" {
		sysroot := os.Getenv("SystemRoot")
		if sysroot == "" {
			sysroot = `C:\Windows`
		}
		return filepath.Join(sysroot, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

// IsSudoAvailable returns true on Unix when `sudo` is in PATH.
func IsSudoAvailable() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	_, err := exec.LookPath("sudo")
	return err == nil
}

// CanRunSudoWithoutPassword probes `sudo -n true`. Returns true when
// passwordless sudo is configured (NOPASSWD) or we're root.
func CanRunSudoWithoutPassword() bool {
	if runtime.GOOS == "windows" {
		return true
	}
	if !IsSudoAvailable() {
		// Container without sudo: only OK when we're already uid 0.
		return os.Geteuid() == 0
	}
	c := exec.Command("sudo", "-n", "true")
	return c.Run() == nil
}

// AddDNSEntries appends marker-wrapped block of "127.0.0.1 <host>" lines.
// idempotent: if the marker block already exists it is replaced (not duplicated).
func AddDNSEntries(hosts []string) error {
	path := HostsFilePath()
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}
	newContent := buildHostsContent(content, hosts)
	return writeHosts(path, content, newContent)
}

// RemoveAllDNSEntries strips the entire flow_router marker block.
func RemoveAllDNSEntries() error {
	path := HostsFilePath()
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}
	newContent := buildHostsContent(content, nil)
	if bytes.Equal(content, newContent) {
		return nil
	}
	return writeHosts(path, content, newContent)
}

// CheckDNSStatus returns a map host→isHijacked (true when entry points to 127.0.0.1).
func CheckDNSStatus(hosts []string) (map[string]bool, error) {
	content, err := os.ReadFile(HostsFilePath())
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, h := range hosts {
		out[h] = bytes.Contains(content, []byte("127.0.0.1 "+h)) ||
			bytes.Contains(content, []byte("127.0.0.1\t"+h))
	}
	return out, nil
}

// buildHostsContent strips any existing marker block from content and (when
// hosts is non-nil) appends a fresh one with the given hosts.
func buildHostsContent(content []byte, hosts []string) []byte {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var out bytes.Buffer
	inBlock := false
	for scanner.Scan() {
		ln := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(ln), dnsMarker) {
			inBlock = true
			continue
		}
		if inBlock && strings.HasPrefix(strings.TrimSpace(ln), dnsMarkerEnd) {
			inBlock = false
			continue
		}
		if inBlock {
			continue
		}
		out.WriteString(ln + "\n")
	}
	// Trim trailing blank lines so re-applying produces byte-identical output
	// (idempotent across repeated AddDNSEntries calls).
	kept := bytes.TrimRight(out.Bytes(), "\n")
	final := bytes.NewBuffer(kept)
	final.WriteByte('\n')
	if len(hosts) > 0 {
		final.WriteString("\n")
		final.WriteString(dnsMarker + "\n")
		for _, h := range hosts {
			final.WriteString("127.0.0.1 " + h + "\n")
		}
		final.WriteString(dnsMarkerEnd + "\n")
	}
	return final.Bytes()
}

// writeHosts dispatches the actual write per OS. On Unix we attempt direct
// write first (works if hosts is owned by current user, e.g. in containers);
// fall back to `sudo tee` when not writable. On Windows we use an atomic
// rename strategy with backup.
func writeHosts(path string, original, newContent []byte) error {
	if runtime.GOOS == "windows" {
		return writeHostsWindowsAtomic(path, original, newContent)
	}
	// Unix: try direct write
	if err := os.WriteFile(path, newContent, 0o644); err == nil {
		return nil
	}
	// Try sudo (passwordless)
	if !IsSudoAvailable() {
		return fmt.Errorf("hosts file requires elevation and sudo is unavailable")
	}
	tmp, err := os.CreateTemp("", "flow_hosts_*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(newContent); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	c := exec.Command("sudo", "-n", "cp", tmp.Name(), path)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo cp hosts: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writeHostsWindowsAtomic(target string, originalContent, newContent []byte) error {
	tmpNew := target + ".flow_router.new"
	tmpBak := target + ".flow_router.bak"
	if err := os.WriteFile(tmpNew, newContent, 0o600); err != nil {
		return fmt.Errorf("write tmp.new: %w", err)
	}
	_ = os.Remove(tmpBak)
	if err := os.Rename(target, tmpBak); err != nil {
		_ = os.Remove(tmpNew)
		return fmt.Errorf("rename target→.bak: %w", err)
	}
	if err := os.Rename(tmpNew, target); err != nil {
		// Rollback: restore .bak → target. If that fails too, hand-write original.
		if rerr := os.Rename(tmpBak, target); rerr != nil {
			_ = os.WriteFile(target, originalContent, 0o600)
		}
		_ = os.Remove(tmpNew)
		return fmt.Errorf("rename .new→target: %w", err)
	}
	_ = os.Remove(tmpBak)
	return nil
}
