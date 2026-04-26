#!/usr/bin/env bash
set -euo pipefail

# Locate the repository root relative to this script so the bootstrap works
# regardless of the caller's working directory.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GO_BIN="${TSGOLINT_GO_BIN:-go}"
GO_VERSION_MIN="${TSGOLINT_GO_VERSION_MIN:-1.26}"
FORK_PATH="${TSGOLINT_FORK_PATH:-$REPO_ROOT/../typescript-go}"
BIN_DIR="${TSGOLINT_BIN_DIR:-$REPO_ROOT/bin}"

# TSGOLINT_BOOTSTRAP_PHASE controls how far the bootstrap runs:
#   check  - verify prerequisites only
#   build  - prerequisites + build the binary  (default)
#   smoke  - prerequisites + build + smoke lint  (added in a later scenario)
PHASE="${TSGOLINT_BOOTSTRAP_PHASE:-build}"

fail() {
	echo "ERROR: $*" >&2
	exit 1
}

# version_lt returns 0 (true) when $1 is strictly less than $2 in dotted version order.
version_lt() {
	[ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$1" ]
}

phase_check() {
	if ! command -v "$GO_BIN" >/dev/null 2>&1; then
		fail "prerequisite 'Go' not found: '$GO_BIN' is not on PATH or is not executable"
	fi

	local raw
	raw="$("$GO_BIN" version 2>/dev/null || true)"
	if [ -z "$raw" ]; then
		fail "prerequisite 'Go' detected at '$GO_BIN' but 'go version' produced no output"
	fi

	# Parse the version only when the line begins with the canonical "go version go"
	# prefix. This rejects wrapper output (asdf, mise) that prepends noise.
	local detected
	detected="$(printf '%s' "$raw" | grep -Eo '^go version go[0-9]+\.[0-9]+(\.[0-9]+)?' | sed -E 's/^go version go//')"
	if [ -z "$detected" ]; then
		fail "prerequisite 'Go' version could not be parsed from: $raw"
	fi

	if version_lt "$detected" "$GO_VERSION_MIN"; then
		fail "prerequisite 'Go' version $detected is below minimum $GO_VERSION_MIN"
	fi
	echo "Go toolchain: $detected (OK)"

	if [ ! -d "$FORK_PATH" ]; then
		fail "prerequisite 'typescript-go fork' not found at '$FORK_PATH' (set TSGOLINT_FORK_PATH to override)"
	fi
	echo "typescript-go fork: $FORK_PATH (OK)"
}

phase_build() {
	mkdir -p "$BIN_DIR"
	echo "Building tsgolint binary..."
	(cd "$REPO_ROOT" && "$GO_BIN" build -o "$BIN_DIR/tsgolint" ./cmd/tsgolint)
	echo "Build complete: $BIN_DIR/tsgolint"
}

# phase_smoke runs the freshly built binary against a checked-in fixture
# project that contains a known floating-promise violation. A successful
# smoke run produces exit code 1 (lint diagnostics found) and emits at
# least one diagnostic in JSON format.
phase_smoke() {
	local fixture="$REPO_ROOT/testdata/fixtures/smoke"
	echo "Running smoke lint against $fixture ..."
	local output
	output=$("$BIN_DIR/tsgolint" --format json "$fixture/main.ts" 2>&1) || true
	if printf '%s' "$output" | grep -q '"ruleId":"no-floating-promises"'; then
		echo "Smoke lint produced expected diagnostic."
	else
		fail "smoke lint did not produce the expected diagnostic. Output:\n$output"
	fi
}

phase_check

case "$PHASE" in
	check)
		;;
	build)
		phase_build
		;;
	smoke)
		phase_build
		phase_smoke
		;;
	*)
		fail "unknown TSGOLINT_BOOTSTRAP_PHASE '$PHASE' (expected: check, build, smoke)"
		;;
esac

echo "Bootstrap complete."
