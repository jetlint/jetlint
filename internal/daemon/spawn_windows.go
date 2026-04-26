//go:build windows

package daemon

import (
	"os"
	"syscall"
)

// Windows lacks an exact flock equivalent. The PID file already serves
// as a marker; relying on POSIX-style advisory locking is unnecessary
// when the surrounding orchestration (deterministic socket path,
// reachable() probe) already converges on a single live daemon.
//
// flockExclusive returns nil immediately on Windows; correctness is
// preserved by the caller's reachable-probe re-check.
func flockExclusive(f *os.File) error { return nil }
func unflock(f *os.File)              {}

// detachAttr returns SysProcAttr settings that start the daemon in its
// own process group on Windows, so the parent can exit without taking
// the daemon down.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: 0x00000008, // DETACHED_PROCESS
	}
}
