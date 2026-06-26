# VirtBBS — AI assistant notes

Guidance for Claude, Cursor, and other coding agents working in this repository.

## Repository location

The repo may live on an external volume (e.g. `/Volumes/JohnDovey/Projects/BBS/VirtBBS`). Toolchains are installed on the **host system**, not on that volume.

## .NET SDK (macOS)

| Item | Path |
|------|------|
| `dotnet` CLI | `/usr/local/share/dotnet/dotnet` |
| SDKs | `/usr/local/share/dotnet/sdk/` |
| Runtimes | `/usr/local/share/dotnet/shared/` |
| User cache | `~/.dotnet/` |

This repo pins **.NET 8** via `global.json` (SDK 8.0.203). Projects target `net8.0`.

If `dotnet` is not found, ensure PATH includes:

```bash
export PATH="/usr/local/share/dotnet:$HOME/.dotnet/tools:$PATH"
```

Or invoke the full path: `/usr/local/share/dotnet/dotnet build …`

## .NET projects

| Project | Directory | Target | macOS build? |
|---------|-----------|--------|--------------|
| Sysop GUI (Avalonia) | `gui-dotnet/VirtBBS.GUI/` | `net8.0` | Yes |
| Terminal client (WinForms) | `dotnet-virtterm/VirtTerm/` | `net8.0-windows` | No — Windows only |

### Sysop GUI (primary .NET app on macOS)

```bash
cd gui-dotnet/VirtBBS.GUI
dotnet build
dotnet run
```

### Terminal client

`dotnet-virtterm` uses WinForms (`net8.0-windows`). Do **not** attempt to build or run it on macOS/Linux. Build on Windows instead.

## Go server

The BBS server is Go (no cgo). See `BUILDING.md` for full instructions.

```bash
go build ./cmd/virtbbs
./virtbbs -config VirtBBS.DAT
```

## Common mistakes to avoid

- Assuming the .NET SDK is on the same drive as the repo — it is on the system install path above.
- Trying to build `dotnet-virtterm` on macOS — it requires Windows.
- Using a .NET SDK older than 8 for GUI work — use 8.0.203 (or newer 8.x with `rollForward`).