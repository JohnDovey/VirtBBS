# AnsiEdit

Standalone fullscreen ANSI art editor for VirtBBS and classic BBS screens.

Not a door game — run it locally in a terminal against `.ANS` / `.ASC` files (for example under `../display/`).

## Build

```bash
cd AnsiEdit
GOTOOLCHAIN=local go build -o ansiedit .
```

### Cross-compile (Linux / Windows / macOS)

AnsiEdit is pure Go (`CGO_ENABLED=0`), so targets build from any host:

```bash
cd AnsiEdit
./build-release.sh
```

Zips land in `/Volumes/JohnDovey/tmp/ansiedit-release-<version>/` (override with `RELEASE_DIR`).

Targets: `linux-amd64`, `linux-arm64`, `windows-amd64`, `windows-arm64`, plus `darwin-amd64` / `darwin-arm64`.

One-off examples:

```bash
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o ansiedit .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ansiedit.exe .
```

Windows needs a real TTY console (Windows Terminal / ConPTY). Classic `cmd.exe` may work; SyncTerm or a similar client is better for truecolor ANSI.

## Usage

```bash
./ansiedit                     # new 80×25 canvas
./ansiedit ../display/LOGON.ANS
./ansiedit -cols 80 -rows 40
./ansiedit -version
```

Requires an interactive TTY. On macOS Terminal / iTerm, CP437 glyphs are shown as Unicode. SyncTerm (UTF-8 + truecolor) is ideal for imported HBFS art.

## Keys

| Key | Action |
|-----|--------|
| Arrows, Home/End, PgUp/PgDn | Move / scroll |
| Printable chars | Paint with current FG/BG |
| Space | Paint current draw glyph (default █) |
| Backspace / Del | Clear cell |
| `F` then `0`–`f` | Classic foreground (0–15) |
| `B` then `0`–`7` | Classic background |
| Tab / Shift+Tab | Cycle draw glyph |
| `C` | Character picker |
| `S` / F2 | Save |
| `O` / F3 | Open |
| `I` | Import image (PNG/JPEG/GIF/WebP) |
| `U` | Undo last paint/clear (25 deep) |
| `M` | SAUCE + COMNT editor |
| `N` | New canvas |
| Ctrl+L | Redraw |
| Ctrl+Q / Esc Esc | Quit (confirm if dirty) |
| `?` / F1 | Help |

Letter commands are **uppercase** so lowercase letters can be painted.

## Import image

`I` prompts for path, mode, and width:

- **ANSI (HBFS)** — 2×2 semigraphics + truecolor `38;2` / `48;2` (see [HBFS ANSI Art](https://hbfs.wordpress.com/2017/11/14/ansi-art/))
- **ASCII** — greyscale ramp `.:-=+*#%@`

The whole image is **scaled to fit** inside a max width×height cell box (defaults 80×25), correcting for ~2:1 terminal character aspect. Nothing is cropped from the source.

Sets SAUCE width/height and a COMNT line noting the import.

## SAUCE

`M` edits Title, Author, Group, Date, DataType/FileType, dimensions, Flags, and the full COMNT block (up to 255 × 64-byte lines). Ctrl+S applies; `X` clears SAUCE for the next save.

Files are stored as art bytes + `0x1A` + optional `COMNT` + 128-byte `SAUCE00`.

## Version

Current version: **1.0.4** (patch bump on every change).
