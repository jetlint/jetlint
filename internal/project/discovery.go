// Package project resolves which TypeScript program governs a given file.
// It walks the filesystem from a target path upward, returning the nearest
// tsconfig.json. Resolution is deliberately decoupled from lint
// configuration; the two cascades are independent in this linter.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound is returned by FindNearestTsconfig when no tsconfig.json exists
// in the target's directory or any of its ancestors.
var ErrNotFound = errors.New("no tsconfig.json found in target directory or any ancestor")

// IsNotFound reports whether the given error indicates a missing tsconfig.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// FindNearestTsconfig returns the absolute path of the tsconfig.json that
// governs target. It walks upward from target's directory until a
// tsconfig.json is found, returning ErrNotFound if the walk reaches the
// filesystem root without finding one.
//
// target may be a file or a directory. Symlinks are not followed; the result
// is in terms of the literal directory ancestry as observed by the OS.
func FindNearestTsconfig(target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", target, err)
	}

	// Start the walk from the directory containing target. If target is itself
	// a directory, start there.
	dir := abs
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		dir = filepath.Dir(abs)
	}

	for {
		candidate := filepath.Join(dir, "tsconfig.json")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root.
			return "", fmt.Errorf("for target %s: %w", target, ErrNotFound)
		}
		dir = parent
	}
}
