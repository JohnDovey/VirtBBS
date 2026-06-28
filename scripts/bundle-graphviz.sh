#!/usr/bin/env bash
# bundle-graphviz.sh — copy Graphviz dot + runtime libraries into ./graphviz/
# next to the virtbbs binary for portable diagram rendering.
#
# Usage:
#   ./scripts/bundle-graphviz.sh [DEST_DIR]
#
# DEST_DIR defaults to the current directory. Creates:
#   graphviz/bin/dot
#   graphviz/lib/              (libgvc, libcgraph, …)
#   graphviz/lib/graphviz/     (config8 + libgvplugin_*.dylib — required for PNG)
#
# Requires Graphviz already installed (brew, apt, etc.) unless GRAPHVIZ_PREFIX
# points at an existing prefix (e.g. /opt/homebrew/opt/graphviz).

set -euo pipefail

DEST="${1:-.}"
GV="$DEST/graphviz"
BIN="$GV/bin"
LIB="$GV/lib"
PLUGIN="$LIB/graphviz"

mkdir -p "$BIN" "$PLUGIN"

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

PREFIX="$(dirname "$(dirname "$DOT_SRC")")"

cp "$DOT_SRC" "$BIN/$NAME"
chmod +x "$BIN/$NAME" 2>/dev/null || true

copy_file() {
	local src="$1"
	[[ -f "$src" ]] || return 0
	local base dest
	base="$(basename "$src")"
	dest="$LIB/$base"
	[[ -e "$dest" ]] && return 0
	cp "$src" "$dest"
}

# macOS: copy direct non-system dylib dependencies of one binary (one level).
copy_direct_otool_deps() {
	local bin="$1"
	local dep
	[[ -f "$bin" ]] || return 0
	while IFS= read -r dep; do
		[[ -z "$dep" ]] && continue
		[[ "$dep" == @* ]] && continue
		[[ "$dep" == /usr/lib/* ]] && continue
		[[ "$dep" == /System/* ]] && continue
		[[ -f "$dep" ]] && copy_file "$dep"
	done < <(otool -L "$bin" 2>/dev/null | tail -n +2 | awk '{print $1}')
}

copy_libs() {
	local libs=("$@")
	for lib in "${libs[@]}"; do
		copy_file "$lib"
	done
}

case "$(uname -s)" in
Darwin)
	shopt -s nullglob
	copy_libs \
		"$PREFIX/lib"/libgvc*.dylib \
		"$PREFIX/lib"/libcgraph*.dylib \
		"$PREFIX/lib"/libcdt*.dylib \
		"$PREFIX/lib"/libpathplan*.dylib \
		"$PREFIX/lib"/libxdot*.dylib
	# Plugins + config8 — without this directory dot reports "Format png not recognized".
	if [[ -d "$PREFIX/lib/graphviz" ]]; then
		cp -R "$PREFIX/lib/graphviz/." "$PLUGIN/"
	fi
	# libltdl and direct deps of dot/plugins (pango, gd, … may remain on the host).
	copy_direct_otool_deps "$BIN/$NAME"
	for plug in "$PLUGIN"/*.dylib; do
		[[ -f "$plug" ]] || continue
		copy_direct_otool_deps "$plug"
	done
	# Homebrew libtool is a common direct dep of dot.
	for ltdl in "$PREFIX/../libtool/lib/libltdl"*.dylib /usr/local/opt/libtool/lib/libltdl*.dylib /opt/homebrew/opt/libtool/lib/libltdl*.dylib; do
		[[ -f "$ltdl" ]] && copy_file "$ltdl"
	done
	;;
Linux)
	if command -v ldd >/dev/null 2>&1; then
		while IFS= read -r lib; do
			[[ -n "$lib" && -f "$lib" ]] && copy_file "$lib" || true
		done < <(ldd "$DOT_SRC" | awk '/=>/ {print $3}' | grep -v '^$')
	fi
	if [[ -d "$PREFIX/lib/graphviz" ]]; then
		cp -R "$PREFIX/lib/graphviz/." "$PLUGIN/"
	fi
	if [[ -d "$PREFIX/lib/x86_64-linux-gnu/graphviz" ]]; then
		cp -R "$PREFIX/lib/x86_64-linux-gnu/graphviz/." "$PLUGIN/"
	fi
	;;
MINGW*|MSYS*|CYGWIN*)
	SRC_BIN="$(dirname "$DOT_SRC")"
	shopt -s nullglob
	copy_libs "$SRC_BIN"/*.dll
	if [[ -d "$PREFIX/lib/graphviz" ]]; then
		cp -R "$PREFIX/lib/graphviz/." "$PLUGIN/"
	fi
	;;
esac

smoke_env=()
case "$(uname -s)" in
Darwin) smoke_env=(env "DYLD_LIBRARY_PATH=$LIB") ;;
Linux) smoke_env=(env "LD_LIBRARY_PATH=$LIB") ;;
esac

SMOKE_DOT="$GV/.smoketest.dot"
SMOKE_PNG="$GV/.smoketest.png"
echo 'digraph smoketest { zone -> host_1 -> m_1_0 }' >"$SMOKE_DOT"
if ! "${smoke_env[@]}" "$BIN/$NAME" -Tpng "$SMOKE_DOT" -o "$SMOKE_PNG" 2>"$GV/.smoketest.err"; then
	echo "error: bundled dot failed PNG smoke test:" >&2
	cat "$GV/.smoketest.err" >&2 || true
	rm -f "$SMOKE_DOT" "$SMOKE_PNG" "$GV/.smoketest.err"
	exit 1
fi
rm -f "$SMOKE_DOT" "$SMOKE_PNG" "$GV/.smoketest.err"

echo "Bundled Graphviz into $GV"
echo "  dot: $BIN/$NAME"
if compgen -G "$PLUGIN/*" >/dev/null; then
	echo "  plugins: $PLUGIN/"
fi
if compgen -G "$LIB/*.dylib" >/dev/null || compgen -G "$LIB/*.so*" >/dev/null; then
	echo "  lib: $LIB/"
fi
"${smoke_env[@]}" "$BIN/$NAME" -V 2>/dev/null || true
