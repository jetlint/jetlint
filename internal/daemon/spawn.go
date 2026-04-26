package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SpawnConfig parameterizes EnsureRunning. It is passed by callers (the CLI)
// so the spawn behavior is testable without running the production binary.
type SpawnConfig struct {
	// SocketPath is the Unix domain socket the daemon will listen on.
	SocketPath string

	// PIDFilePath is the path to the file used as the spawn-election lock
	// and to record the daemon PID once started. Defaults to SocketPath
	// with ".pid" appended when empty.
	PIDFilePath string

	// Executable is the program to exec for the daemon process. Defaults
	// to /proc/self/exe (or runtime equivalent).
	Executable string

	// Args are the arguments passed to Executable. Callers typically include
	// a "--daemon" flag and the socket path.
	Args []string

	// LogPath, when non-empty, names a file that receives the spawned
	// daemon's stderr output. The file is opened in append mode and
	// created if it does not exist. When empty, the daemon's stderr is
	// discarded.
	LogPath string

	// HealthProbe is the time budget for the per-attempt health probe used
	// to decide whether an existing socket file represents a live daemon.
	// 0 selects a reasonable default.
	HealthProbe time.Duration

	// SpawnWait is the maximum time the loser of a spawn election waits
	// for the winner's socket to become reachable. 0 selects a reasonable
	// default.
	SpawnWait time.Duration
}

// EnsureRunning makes sure exactly one daemon is running for the configured
// socket path. It returns nil when a daemon is reachable (whether already
// running or freshly spawned by this call), or an error explaining why no
// daemon could be made available.
//
// Concurrency: when two callers race here, both compute the same PID-file
// path; flock on that file elects exactly one spawner. The loser waits up
// to SpawnWait for the winner's socket to become reachable.
func EnsureRunning(ctx context.Context, cfg SpawnConfig) error {
	cfg = cfg.withDefaults()

	if reachable(cfg.SocketPath, cfg.HealthProbe) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cfg.PIDFilePath), 0o700); err != nil {
		return fmt.Errorf("prepare pid-file directory: %w", err)
	}

	pidFile, err := os.OpenFile(cfg.PIDFilePath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open pid file: %w", err)
	}
	defer pidFile.Close()

	if err := flockExclusive(pidFile); err != nil {
		return fmt.Errorf("lock pid file: %w", err)
	}
	defer unflock(pidFile)

	// Re-check after acquiring the lock; another spawner may have completed
	// while we were waiting on flock.
	if reachable(cfg.SocketPath, cfg.HealthProbe) {
		return nil
	}

	// Best-effort cleanup of a stale socket file: if a previous daemon
	// crashed, the socket may exist but no process is listening. Remove it
	// so the new daemon can bind.
	if _, err := os.Stat(cfg.SocketPath); err == nil {
		_ = os.Remove(cfg.SocketPath)
	}

	cmd := exec.Command(cfg.Executable, cfg.Args...)
	// Detach the daemon so it survives the parent CLI's exit.
	cmd.SysProcAttr = detachAttr()
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open /dev/null: %w", err)
	}
	defer devNull.Close()
	cmd.Stdin = devNull
	cmd.Stdout = devNull

	// Daemon stderr is the log channel. Route it to the configured log file
	// when one is supplied so lifecycle events are captured for diagnosis;
	// otherwise drop it to keep the host environment clean.
	if cfg.LogPath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.LogPath), 0o700); err != nil {
			return fmt.Errorf("prepare log directory: %w", err)
		}
		logFile, err := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		// The child process inherits this file descriptor; we close our copy
		// once the spawn completes below.
		defer logFile.Close()
		cmd.Stderr = logFile
	} else {
		cmd.Stderr = devNull
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn daemon: %w", err)
	}

	// Record the spawned PID in the pid file.
	_ = pidFile.Truncate(0)
	_, _ = pidFile.Seek(0, 0)
	_, _ = fmt.Fprintf(pidFile, "%d\n", cmd.Process.Pid)
	_ = pidFile.Sync()

	// Wait for the new daemon to publish its socket and answer health.
	deadline := time.Now().Add(cfg.SpawnWait)
	for time.Now().Before(deadline) {
		if reachable(cfg.SocketPath, cfg.HealthProbe) {
			// Once the daemon is up, we no longer need to keep the spawned
			// process attached; release the parent reference.
			_ = cmd.Process.Release()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	return errors.New("spawned daemon did not become reachable within SpawnWait")
}

// reachable returns true when a daemon at socketPath answers a Ping within
// timeout. It treats any failure (refused, EOF, deadline) as not reachable.
func reachable(socketPath string, timeout time.Duration) bool {
	if _, err := os.Stat(socketPath); err != nil {
		return false
	}
	resp, err := Ping(socketPath, timeout)
	if err != nil {
		return false
	}
	return resp.Kind == KindPing && resp.Error == ""
}

func (c SpawnConfig) withDefaults() SpawnConfig {
	if c.PIDFilePath == "" {
		c.PIDFilePath = strings.TrimSuffix(c.SocketPath, ".sock") + ".pid"
	}
	if c.Executable == "" {
		c.Executable = "/proc/self/exe"
	}
	if c.HealthProbe <= 0 {
		c.HealthProbe = 250 * time.Millisecond
	}
	if c.SpawnWait <= 0 {
		c.SpawnWait = 5 * time.Second
	}
	return c
}

// Platform-specific implementations of flockExclusive, unflock, and
// detachAttr live in spawn_unix.go and spawn_windows.go.
