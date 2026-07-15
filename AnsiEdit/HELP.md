# AnsiEdit User Manual

**AnsiEdit** is a standalone fullscreen ANSI art editor for classic BBS `.ANS` / `.ASC` screens. It ships with [VirtBBS](https://github.com/JohnDovey/VirtBBS) under `AnsiEdit/` but is **not** a BBS door — you run it locally in a terminal.

| | |
|---|---|
| **Version** | 1.0.9 |
| **Copyright** | Copyright (c) 2026 John Dovey \<dovey.john@gmail.com\> |
| **License** | MIT |
| **Repository** | [github.com/JohnDovey/VirtBBS](https://github.com/JohnDovey/VirtBBS) (`AnsiEdit/`) |

In the editor, press **`A`** for an About screen, or **`?` / F1** for a short key cheat sheet. This document is the full manual.

---

## Table of contents

1. [Requirements](#requirements)
2. [Build and run](#build-and-run)
3. [Screen layout](#screen-layout)
4. [Keyboard reference](#keyboard-reference)
5. [Painting and colors](#painting-and-colors)
6. [Undo](#undo)
7. [Insert text (fonts)](#insert-text-fonts)
8. [Resize canvas](#resize-canvas)
9. [Open, save, and new](#open-save-and-new)
10. [Import image](#import-image)
11. [SAUCE and COMNT](#sauce-and-comnt)
12. [File format notes](#file-format-notes)
13. [Release packaging](#release-packaging)
14. [Tips and troubleshooting](#tips-and-troubleshooting)

---

## Requirements

- An **interactive TTY** (real terminal): macOS Terminal, iTerm2, Windows Terminal, SyncTerm, etc.
- On **Windows**, use Windows Terminal / ConPTY (or SyncTerm). Plain legacy `cmd.exe` may be limited.
- For **truecolor** image imports and accurate palette display, prefer SyncTerm or a modern terminal with 24-bit color.
- On macOS/Linux terminals, CP437 glyphs are shown as **Unicode** equivalents.

---

## Build and run

### Local build

```bash
cd AnsiEdit
GOTOOLCHAIN=local go build -o ansiedit .
./ansiedit                  # new 80×25 canvas
./ansiedit ../display/LOGON.ANS
./ansiedit -cols 80 -rows 40
./ansiedit -version
```

Flags:

| Flag | Meaning |
|------|---------|
| `-version` | Print version, copyright, and GitHub URL |
| `-cols N` | Columns for a **new** blank canvas (default 80) |
| `-rows N` | Rows for a **new** blank canvas (default 25) |

Opening an existing file uses that file’s size (see also **Z** resize).

### Cross-compile

AnsiEdit is pure Go (`CGO_ENABLED=0`):

```bash
./build-release.sh    # linux/windows/darwin amd64+arm64 zips
./bundle-release.sh   # one zip: three amd64 binaries + HELP.txt + README.md
```

One-offs:

```bash
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o ansiedit .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ansiedit.exe .
```

---

## Screen layout

Top to bottom:

1. **Title bar** — version, filename, dirty flag `*`, cursor position, canvas size (`cols×rows`), undo depth  
2. **Help strip** — short command reminders  
3. **Art frame** — cyan box border around the editable canvas  
4. **Art viewport** — scrolls vertically with PgUp/PgDn; cursor shown inverted  
5. **Right ruler** — row measure (ticks; numbers every 5 / 10 rows)  
6. **Bottom ruler** — column measure (ticks; tens digit every 10 columns)  
7. **Palette** — sixteen FG colors as colored `█` blocks + white labels `0`–`9` / `a`–`f`  
8. **Status bar** — current FG/BG/glyph, or the last action message  

Default canvas size is **80×25** (classic BBS screen).

---

## Keyboard reference

**Letter commands are UPPERCASE.** Lowercase letters paint that character.

| Key | Action |
|-----|--------|
| Arrows | Move cursor |
| Home / End | Start / end of row |
| PgUp / PgDn | Scroll viewport by page |
| Printable characters | Paint with current FG/BG |
| Space | Paint current **draw glyph** (default `█`) |
| Backspace / Del | Clear cell (space + default attrs) |
| Tab / Shift+Tab | Cycle draw glyph set |
| `C` | Character picker overlay |
| `F` then `0`–`9` or `a`–`f` | Set classic **foreground** (0–15) |
| `B` then `0`–`7` | Set classic **background** (0–7) |
| `U` | Undo (see [Undo](#undo)) |
| `T` | Insert styled text |
| `Z` | Resize canvas |
| `I` | Import image |
| `M` | SAUCE / COMNT editor |
| `O` / F3 | Open file |
| `S` / F2 | Save |
| `N` | New blank canvas |
| `A` | About |
| `?` / F1 | In-editor help |
| Ctrl+L | Force redraw |
| Ctrl+Q | Quit (confirm if dirty) |
| Esc Esc | Quit (confirm if dirty) |
| Esc | Cancel most dialogs |

---

## Painting and colors

### Draw glyph

- **Space** paints the current draw glyph (default full block `█`).  
- **Tab** / **Shift+Tab** cycle a curated CP437 set (blocks, box lines, shading).  
- **`C`** opens a grid picker; Enter selects, Esc cancels.

### Colors

- Classic **16** foregrounds (`F` + digit) and **8** backgrounds (`B` + digit).  
- The bottom **palette** shows each FG index in its real color. Black (`0`) is drawn on a light background so it stays visible. The current FG index’s label is highlighted.  
- Image import can use **truecolor** (`38;2` / `48;2`); the status bar then shows `#rrggbb`.

### Cursor

Movement does not paint. Painting advances the cursor one cell to the right when possible.

---

## Undo

- **`U`** undoes the last paint, clear, or **text insert**.  
- Depth is at least **25** strokes.  
- A whole text stamp (including drop shadow cells) is **one** undo step.  
- Resize, open, and new clear the undo stack.

---

## Insert text (fonts)

Press **`T`**, then:

1. **Text** — string to render (letterforms are uppercase in the built-in fonts).  
2. **Font** — choose by number:  
   - **Block** — solid block letters (VirtBBS-style 5×5)  
   - **Outline** — double-line box style  
   - **Wide** — wider block capitals  
   - **Mini** — compact 3×3  
3. **Size** — `1`–`4` (nearest-neighbor scale).  
4. **Drop shadow** — `Y` stamps a dark-gray copy at +1,+1 behind the text; `N` skips it.

Text is placed at the **current cursor** using the current FG (and BG for filled cells). Esc cancels at any prompt.

---

## Resize canvas

Press **`Z`**. Presets (cols × rows):

| Choice | Size | Notes |
|--------|------|--------|
| 1 | 80×25 | Classic default |
| 2 | 40×25 | Narrow |
| 3 | 80×40 | Tall |
| 4 | 80×50 | Double height |
| 5 | 100×40 | Wide |
| 6 | 132×25 | Wide terminal |
| 7 | 160×50 | Max width band |
| 8 | 160×100 | Large |
| 9 | Custom | Enter width and height |

Limits: width ≤ 160, height ≤ 200. Existing art is **kept where it still fits**; shrinks clip the excess. SAUCE `TInfo1`/`TInfo2` update when SAUCE is present. Undo stack is cleared.

---

## Open, save, and new

### Open (`O` / F3)

- Path is edited in an editable field; leading/trailing spaces are trimmed.  
- On failure, a **popup** shows the error and returns to the path field with your text preserved.  
- Esc cancels. Dirty canvas asks before discard.

### Save (`S` / F2)

- If untitled, prompts for a path (default extension `.ANS` if none of `.ANS` / `.ASC` / `.TXT`).  
- Writes art (+ optional SAUCE trailer) atomically when possible.

### New (`N`)

- Blank **80×25** canvas; clears SAUCE and undo after confirm if dirty.

---

## Import image

Press **`I`**.

1. **Image path** — PNG / JPEG / GIF (first frame) / WebP. Spaces trimmed; errors popup and re-edit path.  
2. **Mode**  
   - **1 — ANSI (HBFS)** — 2×2 semigraphics + truecolor FG/BG ([HBFS ANSI Art](https://hbfs.wordpress.com/2017/11/14/ansi-art/))  
   - **2 — ASCII** — luminance ramp `.:-=+*#%@`  
3. **Max width / height** — defaults **80×25**; the whole image is **scaled to fit** (character aspect ~2:1). Nothing is cropped from the source.  
4. Confirm replace if the canvas is dirty.

Afterwards:

- Canvas becomes the conversion size (within the fit box).  
- SAUCE is prepared with title from the filename, dimensions, and a COMNT line noting the import.  
- Suggested filename `basename.ANS` or `.ASC` if you were untitled.

---

## SAUCE and COMNT

Press **`M`** for the SAUCE editor ([ACiD SAUCE](https://www.acid.org/info/sauce/sauce.htm)).

### Fields

Title (35), Author (20), Group (20), Date (`CCYYMMDD`), DataType, FileType, width (`TInfo1`), height (`TInfo2`), Flags.

### Comments

Up to **255** lines × **64** bytes (`COMNT` block):

- Enter — edit line  
- `I` — insert line  
- `D` — delete line  
- Ctrl+S — apply to editor state (marks dirty)  
- `X` — remove SAUCE on next save  
- Esc — leave (confirm if local changes)

### On disk

Layout: `[art bytes]` + `0x1A` + optional `COMNT` + 128-byte `SAUCE00`.

On load, the trailer is stripped before ANSI parse and kept in editor memory.

---

## File format notes

- **Load** supports CP437 and UTF-8, PCBoard-style `[1;36m` expansion, classic SGR and truecolor `38;2` / `48;2`, common cursor/erase CSI.  
- **Save** emits optimized SGR + CRLF; prefers UTF-8 with Unicode box/blocks for host terminals.  
- Extensions: `.ANS`, `.ASC`, `.TXT`.

VirtBBS `@code@` display macros are **not** expanded in the editor (files can still hold them as literal text for the BBS).

---

## Release packaging

| Script | Output |
|--------|--------|
| `build-release.sh` | Per-platform zips under `/Volumes/JohnDovey/tmp/ansiedit-release-<ver>/` |
| `bundle-release.sh` | Single `AnsiEdit-<ver>.zip` with linux/windows/darwin **amd64** binaries + `HELP.txt` + `README.md` |

Override destination with `RELEASE_DIR=...`.

---

## Tips and troubleshooting

| Issue | What to try |
|-------|-------------|
| “Requires a TTY” | Run in a real terminal, not a piped non-interactive shell |
| Colors wrong / muted | Use SyncTerm or enable truecolor in your terminal |
| Only part of an import visible | Imports fit to max W×H (default 80×25); raise max height with **I**, or **Z** resize after |
| Can’t type uppercase letters | Commands use uppercase; paint lowercase, or insert via **T** / Space glyph |
| Path open failed | Read the popup; edit the same path (spaces are trimmed) |
| Lost edges after resize | Shrinking clips; undo does not restore prior size — save before large shrinks |

---

## Related docs

- [README.md](README.md) — quick start, build, short key table  
- [HELP.txt](HELP.txt) — plain-text cheat sheet bundled in release zips  
- VirtBBS `display/` — sample `.ANS` / `.ASC` screens to open and edit  
