// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Syscall helper (platform-specific).

package mitm

import "syscall"

// syscall_zero returns the platform-correct "signal 0" value used by os.Process
// to test liveness without actually delivering a signal. On Windows, syscall
// has no real signal 0 — passing 0 still triggers FindProcess validation.
func syscall_zero() syscall.Signal { return syscall.Signal(0) }
