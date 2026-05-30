// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — MITM proxy module.

// MITM lifecycle manager.

package mitm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Manager orchestrates the TLS Server + DNS hijack + pidfile bookkeeping.
type Manager struct {
	server *Server
	cm     *CertManager
	addr   string
	hosts  []string
	mu     sync.Mutex
}

// NewManager prepares a Manager bound to addr (typically "127.0.0.1:443"),
// using the given cert manager and hosts to hijack.
func NewManager(addr string, cm *CertManager, hosts []string) *Manager {
	return &Manager{
		addr:  addr,
		cm:    cm,
		hosts: hosts,
	}
}

// PidFile returns the path to the MITM pidfile.
func PidFile() string { return filepath.Join(MITMDir(), ".mitm.pid") }

// Start writes the pidfile, hijacks DNS, then launches the TLS server.
// Returns once the listener is bound; the server runs in the background.
// Caller should call Stop on shutdown.
func (m *Manager) Start(handler interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server != nil {
		return errors.New("manager already started")
	}
	if err := os.MkdirAll(MITMDir(), 0o700); err != nil {
		return fmt.Errorf("mkdir mitm: %w", err)
	}
	if err := os.WriteFile(PidFile(), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return fmt.Errorf("write pidfile: %w", err)
	}
	if len(m.hosts) > 0 {
		if err := AddDNSEntries(m.hosts); err != nil {
			// Pidfile cleanup on DNS failure
			_ = os.Remove(PidFile())
			return fmt.Errorf("dns hijack: %w", err)
		}
	}
	srv := NewServer(m.addr, m.cm, nil) // handler hooked via package-level rewriters
	m.server = srv
	go func() { _ = srv.Start() }()
	return nil
}

// Stop drains the server (5s context) and removes the pidfile and DNS entries.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = m.server.Shutdown(ctx)
	m.server = nil
	_ = RemoveAllDNSEntries()
	_ = os.Remove(PidFile())
	return nil
}

// ReadPidFile returns the recorded pid (or 0 when missing/invalid).
func ReadPidFile() int {
	b, err := os.ReadFile(PidFile())
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return p
}

// IsRunning probes the pidfile and the OS for liveness via os.FindProcess +
// signal 0. Returns true when a live mitm process matches the pidfile.
func IsRunning() bool {
	pid := ReadPidFile()
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, signal 0 tests existence without killing. Windows: FindProcess
	// already returns nil for unknown pids (per docs we can also probe).
	return p.Signal(syscall_zero()) == nil
}
