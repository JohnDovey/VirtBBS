#!/bin/zsh
# Cross-compile AnsiEdit for Linux and Windows (and optional macOS).
# Usage:
#   ./build-release.sh              # all default targets
#   ./build-release.sh 1.0.2        # override version label
#   RELEASE_DIR=/path ./build-release.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

VERSION="${1:-$(grep 'const Version' version.go | sed 's/.*"\(.*\)".*/\1/')}"
OUT="${RELEASE_DIR:-/Volumes/JohnDovey/tmp/ansiedit-release-${VERSION}}"

rm -rf "$OUT"
mkdir -p "$OUT"

echo "==> Building AnsiEdit ${VERSION} -> ${OUT}"

build_one() {
  local goos=$1 goarch=$2 label=$3
  local bin="ansiedit"
  [[ "$goos" == windows ]] && bin="ansiedit.exe"
  local dest="${OUT}/ansiedit-${VERSION}-${label}"
  mkdir -p "$dest"
  echo "  ${label}"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 GOTOOLCHAIN=local \
    go build -trimpath -ldflags="-s -w" -o "${dest}/${bin}" .
  cp README.md "${dest}/"
  (cd "$OUT" && zip -rq "ansiedit-${VERSION}-${label}.zip" "ansiedit-${VERSION}-${label}")
}

# Requested platforms
build_one linux   amd64 linux-amd64
build_one linux   arm64 linux-arm64
build_one windows amd64 windows-amd64
build_one windows arm64 windows-arm64

# macOS (host convenience; same pure-Go build)
build_one darwin  amd64 darwin-amd64
build_one darwin  arm64 darwin-arm64

echo "==> Done:"
ls -lh "$OUT"/*.zip
