# Multi-Relay Chat (MRC)

VirtBBS includes a **built-in** MRC 1.3 client: one process-level hub connection shared by every node, plus a full-screen ANSI UI on Telnet/SSH. This is **not** a DOOR.SYS/PTY door.

## Enable

1. Open **Admin → MRC** (`/admin/mrc`), or edit `VirtBBS.DAT`:

```toml
[mrc]
  enabled      = true
  host         = "mrc.bottomlessabyss.net"
  port         = 5000          # use 5001 with use_tls = true
  use_tls      = false
  bbs_name     = ""            # optional; defaults to sanitised [bbs].name
  bbs_pretty   = ""
  sysop        = ""
  description  = ""
  telnet       = "example.com:2323"
  ssh          = ""
  website      = ""
  default_room = "lobby"
  min_security = 10
```

2. Save. The hub reapplies config immediately (no full process restart). First-time enable from Admin works because the hub supervisor is always started with VirtBBS.

3. From Telnet/SSH main menu, press **`[A]`** (or type `MRC`).

## Network

- **Outbound only** to the MRC relay (default `mrc.bottomlessabyss.net:5000`, or `:5001` TLS). Do not open 5000/5001 inbound on your firewall.
- One TCP session represents your whole BBS; every local chatter attaches through that hub.

## Terminal UI

Inspired by [ANetMRC](https://github.com/anetonline/ANetMRC):

- Row 1 — room, topic, mention badge, handle
- Middle — scrollback (Up/Down to scroll; pipe colors supported)
- Row 24 — input; **ESC** or `/quit` returns to the main menu

While in MRC, local Talk/broadcast is muted so messages do not scramble the screen. Node status shows as chat (`MRC: room`).

### Slash commands (v1)

| Command | Action |
|---------|--------|
| `/join <room>` | Change room |
| `/list` | List rooms |
| `/chatters` / `/who` | Who is in rooms / user list |
| `/whoon` | Network presence |
| `/msg <user> <text>` | Private message |
| `/me <text>` | Action |
| `/topic [text]` | Show or set topic |
| `/motd` | Message of the day |
| `/bbses` | Connected BBSes |
| `/info [n]` | BBS info |
| `/mentions` | Show/reset mention count |
| `/clear` | Clear scrollback |
| `/quit` | Leave MRC |
| `/help` | Short help |

Tab completes nicknames seen in the current room.

## Per-user prefs

Stored in SQLite table `mrc_user_prefs` (handle, colors, twit list, etc.). The first prompt on entry lets the caller confirm/change their MRC handle.

## Troubleshooting

- **Admin status shows reconnecting** — check outbound connectivity to the relay host/port; review process logs for `mrc: session ended`.
- **Insufficient security** — raise the caller’s level or lower `min_security`.
- **Still offline after enable** — wait a few seconds for dial/backoff; status on `/admin/mrc` shows the last error.
