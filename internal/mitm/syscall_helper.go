package mitm

import "syscall"

// syscall_zero returns the platform-correct "signal 0" value used by os.Process
// to test liveness without actually delivering a signal. On Windows, syscall
// has no real signal 0 — passing 0 still triggers FindProcess validation.
func syscall_zero() syscall.Signal { return syscall.Signal(0) }
