#!/usr/bin/env bash
set -euo pipefail

# Locate the repository root relative to this script so the bootstrap works
# regardless of the caller's working directory.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GO_BIN="${TSGOLINT_GO_BIN:-go}"
GO_VERSION_MIN="${TSGOLINT_GO_VERSION_MIN:-1.23}"
FORK_PATH="${TSGOLINT_FORK_PATH:-$REPO_ROOT/../typescript-go}"

fail() {
	echo "ERROR: $*" >&2
	exit 1
}

# Compare two dotted version strings; returns 0 (true) if $1 < $2.
version_lt() {
	[ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$1" ]
}

# --- Go toolchain check ---
if ! command -v "$GO_BIN" >/dev/null 2>&1; then
	fail "prerequisite 'Go' not found: '$GO_BIN' is not on PATH or is not executable"
fi

GO_VERSION_RAW="$("$GO_BIN" version 2>/dev/null || true)"
if [ -z "$GO_VERSION_RAW" ]; then
	fail "prerequisite 'Go' detected at '$GO_BIN' but 'go version' produced no output"
fi

# Parse a line like "go version go1.26.2 linux/amd64" into "1.26.2".
GO_VERSION="$(printf '%s' "$GO_VERSION_RAW" | sed -E 's/^go version go([0-9]+\.[0-9]+(\.[0-9]+)?).*$/\1/')"
if [ -z "$GO_VERSION" ] || [ "$GO_VERSION" = "$GO_VERSION_RAW" ]; then
	fail "prerequisite 'Go' version could not be parsed from: $GO_VERSION_RAW"
fi

if version_lt "$GO_VERSION" "$GO_VERSION_MIN"; then
	fail "prerequisite 'Go' version $GO_VERSION is below minimum $GO_VERSION_MIN"
fi

echo "Go toolchain: $GO_VERSION (OK)"

# --- typescript-go fork check ---
if [ ! -d "$FORK_PATH" ]; then
	fail "prerequisite 'typescript-go fork' not found at '$FORK_PATH' (set TSGOLINT_FORK_PATH to override)"
fi

echo "typescript-go fork: $FORK_PATH (OK)"
echo "Bootstrap prerequisites verified."
