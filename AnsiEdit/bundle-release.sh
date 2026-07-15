#!/bin/zsh
# Build a single AnsiEdit release zip with Linux, Windows, and macOS binaries + HELP.
# Usage:
#   ./bundle-release.sh
#   RELEASE_DIR=/path ./bundle-release.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

VERSION="${1:-$(grep 'const Version' version.go | sed 's/.*"\(.*\)".*/\1/')}"
OUT="${RELEASE_DIR:-/Volumes/JohnDovey/tmp/ansiedit-bundle-${VERSION}}"
NAME="AnsiEdit-${VERSION}"
DIR="${OUT}/${NAME}"

rm -rf "$OUT"
mkdir -p "$DIR"

echo "==> Bundling AnsiEdit ${VERSION} (linux / windows / darwin amd64) -> ${OUT}"

build_bin() {
  local goos=$1 goarch=$2 outfile=$3
  echo "  ${outfile}"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 GOTOOLCHAIN=local \
    go build -trimpath -ldflags="-s -w" -o "${DIR}/${outfile}" .
}

build_bin linux   amd64 "ansiedit-linux-amd64"
build_bin windows amd64 "ansiedit-windows-amd64.exe"
build_bin darwin  amd64 "ansiedit-darwin-amd64"

cp HELP.txt "${DIR}/HELP.txt"
cp HELP.md "${DIR}/HELP.md"
cp README.md "${DIR}/README.md"

(cd "$OUT" && zip -rq "${NAME}.zip" "$NAME")

echo "==> Bundle ready:"
ls -lh "${OUT}/${NAME}.zip"
unzip -l "${OUT}/${NAME}.zip"
echo "${OUT}/${NAME}.zip"
