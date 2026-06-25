# Building VirtBBS

## Prerequisites

- Go 1.22+ (`go version`)
- On macOS: Xcode command-line tools (`xcode-select --install`) — required by Fyne
- On Linux: `libgl1-mesa-dev xorg-dev` packages

## Quick start

```bash
# Fetch dependencies
go mod tidy

# Build the BBS server (no cgo, cross-compiles cleanly)
go build ./cmd/virtbbs

# Build the sysop GUI (requires cgo on native platform for Fyne GL)
go build ./cmd/virtbbs-gui

# Run the BBS server
./virtbbs -config VirtBBS.DAT

# Run the GUI (connect to localhost:9999 by default)
./virtbbs-gui
```

## Connecting

- **Telnet**: `telnet localhost 2323`  (or SyncTerm, NetRunner, etc.)
- **SSH**: `ssh -p 3232 username@localhost`
- **Sysop GUI**: launch `virtbbs-gui`, set host/port/credentials in Settings tab

## Importing from PCBoard 15.3

```bash
# Import users from a PCBoard USERS binary file
./virtbbs -import-users /path/to/USERS

# Import messages from a PCBoard MSGS file into conference 0
./virtbbs -import-msgs /path/to/MSGS -conference 0

# Import config from PCBOARD.DAT
./virtbbs -import-config /path/to/PCBOARD.DAT -out VirtBBS.DAT
```

## Cross-compilation

```bash
# Windows (server only — no cgo)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/virtbbs

# Linux AMD64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/virtbbs

# GUI requires cgo — build natively on target platform
```

## Default ports

| Service | Port |
|---|---|
| Telnet | 2323 |
| SSH | 3232 |
| Sysop API | 9999 |

Change in `VirtBBS.DAT` under `[network]`.
