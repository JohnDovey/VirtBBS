# FREQ (File REQuest)

VirtBBS implements classic FidoNet **FREQ** via netmail to **Freq** (also accepts `FileRequest` / `FREQ` as ToName, or subject `FREQ` with commands in the body — Internet Rex style), plus **WaZOO `.REQ`**, **Bark-style `.REQ`**, **SRIF** control files, and **BinkP session FREQ** via bare-name `M_GET`.

## Responder

When inbound netmail is tossed:

1. **ToName** is `Freq`, `FileRequest`, or `FREQ`, **or** **Subject** is `FREQ` with body commands, **or** FTS **FILE_REQUEST** attribute with filenames in the subject.
2. The sender must be a configured **downlink** or appear in the **nodelist** database.
3. Optional **freq_password** (network config) may be required in subject and/or first body line (same rules as AreaFix).
4. Optional **per-file passwords** via `freq_file_passwords` (`filename = "secret"`); requestors supply `name password` or Bark `name !password`.
5. Matching files are copied as **raw files** into the requester's outbound `ZZZZNNNN.OUT` subdirectory for BinkP pickup.

Inbound **`.REQ`** files (WaZOO/Bark) arriving in the BinkP inbound directory are fulfilled the same way during the session (before outbound send). Bare-name BinkP **`M_GET`** (not FTS-1026 resume) also queues files in-session.

### Commands (one per line)

| Command | Action |
|---------|--------|
| `filename` | Queue that file (searches all active file directories) |
| `filename password` / `filename !password` | Queue with per-file password |
| `*.zip` | Wildcard match (`*` and `?`) |
| `%LIST` / `FILES` | Reply with catalog listing |
| `NODELIST` | Queue latest `NODELIST.*` from nodelist dir |
| `NODEDIFF` | Queue latest `NODEDIFF.*` from nodelist dir |
| `%HELP` | Reply with command summary |

Limits per request: **freq_max_files** (default 5) and **freq_max_bytes** (default 5 MiB).

## Requester

- **Terminal:** FidoNet menu → `[G] FREQ`
- **Web admin:** FidoNet tools → FREQ section
- **API:** compose via `RequestFreq()`

Outbound request formats (per-network `freq_outbound`, overridable per send):

| Format | Use when |
|--------|----------|
| **classic** (default) | Netmail **To: Freq** with commands in the body |
| **file_request** | FTS **FILE_REQUEST** attribute; filenames in subject (BinktermPHP) |
| **wazoo** | WaZOO `.REQ` file in the peer outbound directory |
| **bark** | Bark-style `.REQ` with optional `!password` on each line |

## Configuration (`VirtBBS.DAT` / network admin)

| Key | Default | Description |
|-----|---------|-------------|
| `freq_enabled` | true | Enable FREQ responder |
| `freq_password` | (none) | Optional global password |
| `freq_max_files` | 5 | Max files queued per request |
| `freq_max_bytes` | 5242880 | Max total bytes per request |
| `freq_outbound` | classic | `classic`, `file_request`, `wazoo`, or `bark` |
| `freq_file_passwords` | (none) | Map of filename → password |
| `srif_helper` | (none) | Optional external SRIF helper; empty = built-in parser |

Add **FRQ** to node flags to advertise file-request support in the nodelist.

## BinkP resume

FTS-1026 **`M_GET` resume** (filename size time offset) is honoured on outbound sends: VirtBBS re-announces `M_FILE` at the requested offset and continues. Inbound partial files are also accepted when `M_FILE` carries a non-zero offset.

## Statistics

- **BinkP stats:** `freq_sent` / `freq_recv` (network and per-link), shown on Admin → BinkP
- **Detail tables:** `fido_freq_file_stats`, `fido_freq_node_stats`
- **API:** `GET /admin/fido/freq-stats?network=FidoNet&limit=50` (sysop session required)

## Comparison with BinktermPHP

BinktermPHP supports multiple FREQ transports (WaZOO `.REQ`, netmail `FILE_REQUEST`, BinkP `M_GET`). VirtBBS outbound FREQ defaults to **classic** netmail to **Freq**; set `freq_outbound = "file_request"` (or `wazoo` / `bark`) for other hubs. Inbound accepts netmail, `.REQ`, SRIF, and session `M_GET`, delivering files by raw BinkP outbound `.OUT` pickup.
