// Package transport contains the wire-level concerns of the linter: the
// daemon socket path derivation, framing, and (in future scenarios)
// JSON-RPC envelope handling. It depends only on the standard library so
// both the CLI front-end and the daemon back-end can import it without
// pulling in any rule machinery.
package transport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DaemonSocketPath returns the absolute filesystem path of the Unix domain
// socket the daemon listens on for the given tsconfig. The path is a
// deterministic function of the tsconfig's absolute, cleaned path, so two
// CLI invocations from any working directory compute the same socket.
//
// The path lives under the platform-appropriate runtime directory plus a
// "tsgolint" subdirectory; the directory is not created here. Callers that
// publish the socket are responsible for ensuring the parent directory
// exists.
func DaemonSocketPath(tsconfig string) (string, error) {
	abs, err := filepath.Abs(tsconfig)
	if err != nil {
		return "", fmt.Errorf("absolutize %s: %w", tsconfig, err)
	}
	abs = filepath.Clean(abs)
	digest := sha256.Sum256([]byte(abs))
	name := hex.EncodeToString(digest[:8]) + ".sock"
	return filepath.Join(runtimeDir(), "tsgolint", name), nil
}

// LogPath returns the absolute filesystem path of the per-tsconfig daemon
// log file. The path is a deterministic function of the tsconfig path so
// any consumer (CLI, test, operator) can locate the log without coordination
// with the daemon.
//
// The path lives under the platform-appropriate state directory plus a
// "tsgolint" subdirectory; the directory is not created here. Callers
// that open the log are responsible for ensuring the parent exists.
func LogPath(tsconfig string) (string, error) {
	abs, err := filepath.Abs(tsconfig)
	if err != nil {
		return "", fmt.Errorf("absolutize %s: %w", tsconfig, err)
	}
	abs = filepath.Clean(abs)
	digest := sha256.Sum256([]byte(abs))
	name := hex.EncodeToString(digest[:8]) + ".log"
	return filepath.Join(stateDir(), "tsgolint", name), nil
}

// runtimeDir returns the platform's runtime directory for ephemeral
// per-user files. On Linux this honors XDG_RUNTIME_DIR with a /tmp fallback;
// on macOS it uses os.TempDir; on Windows it uses LOCALAPPDATA with a
// TempDir fallback.
func runtimeDir() string {
	if v := os.Getenv("XDG_RUNTIME_DIR"); v != "" {
		return v
	}
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return v
		}
	}
	return os.TempDir()
}

// stateDir returns the platform's state directory for persistent per-user
// files (logs, caches that survive reboot). On Linux this honors
// XDG_STATE_HOME with $HOME/.local/state as the documented fallback;
// on Windows it uses LOCALAPPDATA; otherwise os.TempDir.
func stateDir() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return v
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state")
	}
	return os.TempDir()
}
