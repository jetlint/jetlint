//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

func flockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func unflock(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// detachAttr returns SysProcAttr settings that detach the daemon from
// the parent's process group so it survives the CLI's exit. On Unix
// this is a fresh session leader.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
