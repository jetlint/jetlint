package transport_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tommymorgan/tsgolint/internal/transport"
)

func TestDaemonSocketPath_IsDeterministicForGivenTsconfig(t *testing.T) {
	tsconfig := "/some/repo/packages/web/tsconfig.json"
	a, err := transport.DaemonSocketPath(tsconfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := transport.DaemonSocketPath(tsconfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != b {
		t.Errorf("expected determinism, got %s and %s", a, b)
	}
}

func TestDaemonSocketPath_DiffersAcrossDifferentTsconfigs(t *testing.T) {
	a, err := transport.DaemonSocketPath("/repo-one/tsconfig.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := transport.DaemonSocketPath("/repo-two/tsconfig.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == b {
		t.Errorf("expected distinct paths for distinct tsconfigs, got %s for both", a)
	}
}

func TestDaemonSocketPath_LivesUnderTheRuntimeDirectory(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	got, err := transport.DaemonSocketPath("/repo/tsconfig.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, runtimeDir) {
		t.Errorf("expected path under %s, got %s", runtimeDir, got)
	}
	// Sanity: it should also be under a tsgolint subdirectory so we don't
	// pollute the runtime dir's root.
	if filepath.Base(filepath.Dir(got)) != "tsgolint" {
		t.Errorf("expected path under a tsgolint/ subdirectory, got %s", got)
	}
}

func TestLogPath_LivesUnderTheStateDirectoryAndIsKeyedByProject(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)

	a, err := transport.LogPath("/repo-one/tsconfig.json")
	if err != nil {
		t.Fatalf("LogPath: %v", err)
	}
	b, err := transport.LogPath("/repo-two/tsconfig.json")
	if err != nil {
		t.Fatalf("LogPath: %v", err)
	}
	if !strings.HasPrefix(a, stateDir) {
		t.Errorf("expected log path under %s, got %s", stateDir, a)
	}
	if filepath.Base(filepath.Dir(a)) != "tsgolint" {
		t.Errorf("expected log under tsgolint/ subdirectory, got %s", a)
	}
	if a == b {
		t.Errorf("expected distinct log paths for distinct projects, got %s for both", a)
	}
}

func TestDaemonSocketPath_AbsolutizesRelativeTsconfigPath(t *testing.T) {
	// Two callers from different working directories that resolve to the same
	// absolute tsconfig path must compute the same socket path.
	a, err := transport.DaemonSocketPath("/abs/repo/tsconfig.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := transport.DaemonSocketPath("/abs/./repo/./tsconfig.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != b {
		t.Errorf("expected paths to match after absolutization, got %s vs %s", a, b)
	}
}
