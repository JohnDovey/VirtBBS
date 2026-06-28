#!/usr/bin/env bash
# bundle-graphviz.sh — copy Graphviz dot + runtime libraries into ./graphviz/
# next to the virtbbs binary for portable diagram rendering.
#
# Usage:
#   ./scripts/bundle-graphviz.sh [DEST_DIR]
#
# DEST_DIR defaults to the current directory. Creates:
#   graphviz/bin/dot
#   graphviz/lib/   (shared libraries when required by the platform)
#
# Requires Graphviz already installed (brew, apt, etc.) unless GRAPHVIZ_PREFIX
# points at an existing prefix (e.g. /opt/homebrew/opt/graphviz).

set -euo pipefail

DEST="${1:-.}"
GV="$DEST/graphviz"
BIN="$GV/bin"
LIB="$GV/lib"

mkdir -p "$BIN" "$LIB"

dot_name() {
	case "$(uname -s)" in
	MINGW*|MSYS*|CYGWIN*|Windows*) echo "dot.exe" ;;
	*) echo "dot" ;;
	esac
}

NAME="$(dot_name)"
DOT_SRC=""

if [[ -n "${GRAPHVIZ_PREFIX:-}" ]]; then
	DOT_SRC="$GRAPHVIZ_PREFIX/bin/$NAME"
elif command -v dot >/dev/null 2>&1; then
	DOT_SRC="$(command -v dot)"
elif [[ "$(uname -s)" == "Darwin" ]] && command -v brew >/dev/null 2>&1; then
	GRAPHVIZ_PREFIX="$(brew --prefix graphviz 2>/dev/null || true)"
	if [[ -n "$GRAPHVIZ_PREFIX" && -x "$GRAPHVIZ_PREFIX/bin/dot" ]]; then
		DOT_SRC="$GRAPHVIZ_PREFIX/bin/dot"
	fi
fi

if [[ -z "$DOT_SRC" || ! -x "$DOT_SRC" ]]; then
	echo "error: dot not found — install Graphviz or set GRAPHVIZ_PREFIX" >&2
	exit 1
fi

cp "$DOT_SRC" "$BIN/$NAME"
chmod +x "$BIN/$NAME" 2>/dev/null || true

copy_libs() {
	local libs=("$@")
	for lib in "${libs[@]}"; do
		[[ -f "$lib" ]] && cp "$lib" "$LIB/"
	done
}

case "$(uname -s)" in
Darwin)
	PREFIX="$(dirname "$(dirname "$DOT_SRC")")"
	shopt -s nullglob
	copy_libs "$PREFIX/lib"/libgvc*.dylib "$PREFIX/lib"/libcgraph*.dylib "$PREFIX/lib"/libcdt*.dylib \
		"$PREFIX/lib"/libpathplan*.dylib "$PREFIX/lib"/libxdot*.dylib
	;;
Linux)
	if command -v ldd >/dev/null 2>&1; then
		while IFS= read -r lib; do
			[[ -n "$lib" && -f "$lib" ]] && cp "$lib" "$LIB/" || true
		done < <(ldd "$DOT_SRC" | awk '/=>/ {print $3}' | grep -v '^$')
	fi
	;;
MINGW*|MSYS*|CYGWIN*)
	SRC_BIN="$(dirname "$DOT_SRC")"
	shopt -s nullglob
	copy_libs "$SRC_BIN"/*.dll
	;;
esac

echo "Bundled Graphviz into $GV"
echo "  dot: $BIN/$NAME"
if compgen -G "$LIB/*" >/dev/null; then
	echo "  lib: $LIB/"
fi
"$BIN/$NAME" -V 2>/dev/null || true
