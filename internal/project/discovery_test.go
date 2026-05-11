package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jetlint/jetlint/internal/project"
)

// makeTree writes a small directory tree under a temp directory and returns
// its root. Each entry maps a relative path to its file contents (or "" for
// directories).
func makeTree(t *testing.T, entries map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range entries {
		full := filepath.Join(root, rel)
		if content == "" && filepath.Ext(rel) == "" {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", full, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir parent of %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

func TestFindNearestTsconfig_FindsAdjacentConfig(t *testing.T) {
	root := makeTree(t, map[string]string{
		"tsconfig.json": "{}",
		"src/foo.ts":    "export const x = 1;",
	})
	got, err := project.FindNearestTsconfig(filepath.Join(root, "src", "foo.ts"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "tsconfig.json")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestFindNearestTsconfig_PrefersTheClosestConfigInTheTree(t *testing.T) {
	root := makeTree(t, map[string]string{
		"tsconfig.json":           "{}",
		"packages/web/tsconfig.json": `{"extends": "../../tsconfig.json"}`,
		"packages/web/src/page.ts":   "export {};",
	})
	got, err := project.FindNearestTsconfig(filepath.Join(root, "packages", "web", "src", "page.ts"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "packages", "web", "tsconfig.json")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestFindNearestTsconfig_AcceptsDirectoryTargets(t *testing.T) {
	root := makeTree(t, map[string]string{
		"tsconfig.json": "{}",
		"src/foo.ts":    "export {};",
	})
	got, err := project.FindNearestTsconfig(filepath.Join(root, "src"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "tsconfig.json")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestFindNearestTsconfig_ReturnsErrorWhenNoConfigExists(t *testing.T) {
	root := makeTree(t, map[string]string{
		"src/foo.ts": "export {};",
	})
	_, err := project.FindNearestTsconfig(filepath.Join(root, "src", "foo.ts"))
	if err == nil {
		t.Fatal("expected error for missing tsconfig, got nil")
	}
	if !project.IsNotFound(err) {
		t.Errorf("expected ErrNotFound-class error, got %v", err)
	}
}

func TestFindNearestTsconfig_StopsAtFilesystemRoot(t *testing.T) {
	// Targeting a file in /tmp (or wherever t.TempDir lives) without a tsconfig
	// must not walk past the filesystem root.
	root := makeTree(t, map[string]string{
		"src/foo.ts": "",
	})
	_, err := project.FindNearestTsconfig(filepath.Join(root, "src", "foo.ts"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
