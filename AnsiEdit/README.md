# AnsiEdit

Standalone fullscreen ANSI art editor for VirtBBS and classic BBS screens.

Not a door game — run it locally in a terminal against `.ANS` / `.ASC` files (for example under `../display/`).

## Build

```bash
cd AnsiEdit
GOTOOLCHAIN=local go build -o ansiedit .
```

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

Sets SAUCE width/height and a COMNT line noting the import.

## SAUCE

`M` edits Title, Author, Group, Date, DataType/FileType, dimensions, Flags, and the full COMNT block (up to 255 × 64-byte lines). Ctrl+S applies; `X` clears SAUCE for the next save.

Files are stored as art bytes + `0x1A` + optional `COMNT` + 128-byte `SAUCE00`.

## Version

Current version: **1.0.0** (patch bump on every change).
