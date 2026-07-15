# AnsiArt

Convert images to ANSI (truecolor semigraphics) or ASCII art for VirtBBS.

Inspired by [HBFS ANSI Art](https://hbfs.wordpress.com/2017/11/14/ansi-art/).
Outputs include ACiD **SAUCE** metadata.

Current version: **1.0.0**

## Build

```bash
cd DoorGames/AnsiArt
GOTOOLCHAIN=local go build -o ansiart .
```

## Local test

```bash
./ansiart -local
# menu: convert a local image path, or put files in LIBRARY/inbox/
```

## VirtBBS

```toml
[[doors]]
  name             = "AnsiArt"
  description      = "Convert images to ANSI/ASCII art"
  cmd              = "DoorGames/AnsiArt/ansiart"
  work_dir         = "DoorGames/AnsiArt"
  drop_file        = "DOOR.SYS"
  append_drop_file = true
  min_security     = 10
```

Web UI: `/ansiart` for browser upload/convert/download (same library).

Deploy: `/Volumes/JohnDovey/Projects/ServiceMonitor/scripts/sync-binaries.sh`
