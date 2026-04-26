#!/usr/bin/env bash
# Cross-compile tsgolint for the platforms listed in the v0.1 release
# matrix and emit checksums for each artifact. Outputs to ./dist by
# default; override with TSGOLINT_DIST_DIR.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DIST_DIR="${TSGOLINT_DIST_DIR:-$REPO_ROOT/dist}"

VERSION="${TSGOLINT_VERSION:-0.0.0-dev}"
LDFLAGS="-X 'github.com/tommymorgan/tsgolint/internal/cli.Version=$VERSION' -s -w"

# Each entry: GOOS GOARCH binary-suffix
TARGETS=(
	"linux amd64 ''"
	"linux arm64 ''"
	"darwin amd64 ''"
	"darwin arm64 ''"
	"windows amd64 .exe"
)

mkdir -p "$DIST_DIR"

for entry in "${TARGETS[@]}"; do
	# shellcheck disable=SC2206
	parts=($entry)
	goos="${parts[0]}"
	goarch="${parts[1]}"
	suffix="${parts[2]//\'/}"
	out="$DIST_DIR/tsgolint-$VERSION-$goos-$goarch$suffix"
	echo "Building $out ..."
	(
		cd "$REPO_ROOT"
		GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
			go build -ldflags "$LDFLAGS" -trimpath -o "$out" ./cmd/tsgolint
	)
done

echo "Generating SHA-256 checksums ..."
(
	cd "$DIST_DIR"
	# Match anything that looks like a tsgolint artifact for this version.
	sha256sum tsgolint-"$VERSION"-* > "tsgolint-$VERSION-checksums.txt"
)

echo "Release artifacts in $DIST_DIR:"
ls -la "$DIST_DIR"
