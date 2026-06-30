# FREQ (File REQuest)

VirtBBS implements classic FidoNet **FREQ** via netmail to **Freq** (also accepts `FileRequest` / `FREQ` as ToName, or subject `FREQ` with commands in the body — Internet Rex style).

## Responder

When inbound netmail is tossed:

1. **ToName** is `Freq`, `FileRequest`, or `FREQ`, **or** **Subject** is `FREQ` with body commands.
2. The sender must be a configured **downlink** or appear in the **nodelist** database.
3. Optional **freq_password** (network config) may be required in subject and/or first body line (same rules as AreaFix).
4. Matching files are copied as **raw files** into the requester's outbound `ZZZZNNNN.OUT` subdirectory for BinkP pickup.

### Commands (one per line)

| Command | Action |
|---------|--------|
| `filename` | Queue that file (searches all active file directories) |
| `*.zip` | Wildcard match (`*` and `?`) |
| `%LIST` / `FILES` | Reply with catalog listing |
| `NODELIST` | Queue latest `NODELIST.*` from nodelist dir |
| `NODEDIFF` | Queue latest `NODEDIFF.*` from nodelist dir |
| `%HELP` | Reply with command summary |

Limits per request: **freq_max_files** (default 5) and **freq_max_bytes** (default 5 MiB).

## Requester

- **Terminal:** FidoNet menu → `[G] FREQ`
- **Web admin:** FidoNet tools → FREQ section
- **API:** compose netmail via `RequestFreq()`

Outbound requests support two formats (per-network default in `freq_outbound`, overridable per send):

| Format | Use when |
|--------|----------|
| **classic** (default) | Netmail **To: Freq** with commands in the body (VirtBBS, HPT, Squish-style robots) |
| **file_request** | FTS **FILE_REQUEST** attribute (`0x0800`); filenames in subject (BinktermPHP) |

- **Terminal:** `[G] FREQ` — press Enter for network default, or `C` / `F` to override
- **Web admin:** FidoNet tools → FREQ section (format dropdown)
- **Config:** `freq_outbound = "classic"` or `"file_request"` per network

For **classic**, `freq_password` (if set) is sent in the **subject**; commands go in the body.
For **file_request**, filenames are space-separated in the **subject** and optional remote password is on the first body line.

## Configuration (`VirtBBS.DAT` / network admin)

| Key | Default | Description |
|-----|---------|-------------|
| `freq_enabled` | true | Enable FREQ responder |
| `freq_password` | (none) | Optional global password |
| `freq_max_files` | 5 | Max files queued per request |
| `freq_max_bytes` | 5242880 | Max total bytes per request |
| `freq_outbound` | classic | Outbound request format: `classic` or `file_request` |

Add **FRQ** to node flags to advertise file-request support in the nodelist.

## Statistics

- **BinkP stats:** `freq_sent` / `freq_recv` (network and per-link), shown on Admin → BinkP
- **Detail tables:** `fido_freq_file_stats`, `fido_freq_node_stats`
- **API:** `GET /admin/fido/freq-stats?network=FidoNet&limit=50` (sysop session required)

## Not implemented

WaZOO `.REQ`, Bark, SRIF `M_GET`, BinkP `M_GET`, and per-file passwords are **not** supported. VirtBBS uses netmail + raw outbound files only.

## Comparison with BinktermPHP

BinktermPHP supports multiple FREQ transports (WaZOO `.REQ`, netmail `FILE_REQUEST`, BinkP `M_GET`). VirtBBS outbound FREQ defaults to **classic** netmail to **Freq**; set `freq_outbound = "file_request"` (or choose per send) for BinktermPHP hubs. The inbound responder accepts both styles and delivers files by raw BinkP outbound `.OUT` pickup.
