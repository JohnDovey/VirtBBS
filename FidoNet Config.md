# VirtBBS — FidoNet Configuration Guide

This guide covers every FidoNet setting in `VirtBBS.DAT`, how echomail/netmail
routing works, how to add additional FidoNet-compatible networks, AreaFix,
and the PING test utility. It covers VirtBBS **0.2.0**.

---

## 1. Enabling FidoNet

All FidoNet settings live under the `[fido]` table in `VirtBBS.DAT`:

```toml
[fido]
  enabled      = true
  address      = "1:1/1"
  uplink       = "1:1/100"
  password     = ""
  inbound_dir  = "fido/inbound"
  outbound_dir = "fido/outbound"
  nodelist_dir = "fido/nodelist"
  binkp_port   = 24554
  taglines_file = ""

  [fido.areas]
```

| Field | Meaning |
|---|---|
| `enabled` | Master on/off switch. When `false`, all FidoNet menus, the toss/scan/poll commands, and the management API's `fido.*` endpoints refuse to run. |
| `address` | **This BBS's own FidoNet address**, in `zone:net/node` or `zone:net/node.point` form (e.g. `1:234/567` or `1:234/567.1` for a point system). |
| `uplink` | The address of the system this BBS exchanges mail with — your boss node or hub. All routed (non-crash) netmail and all echomail go here. |
| `password` | The session/packet password shared with your uplink. Leave blank if your uplink doesn't require one. |
| `inbound_dir` | Directory `.pkt` files are read from when tossing (see §4). Created automatically if missing. |
| `outbound_dir` | Directory `.pkt` files are written to when scanning/sending (see §5). Created automatically if missing. |
| `nodelist_dir` | Directory containing `NODELIST.*` files for address lookups (sysop name, BBS name, phone, flags) shown in the in-BBS nodelist browser and used by `[I]Ping a node`. |
| `binkp_port` | TCP port used when **polling** your uplink over BinkP. Defaults to `24554` if zero/unset. |
| `taglines_file` | Optional path to a text file, one tagline per line. A random line is inserted above the tear line on every outgoing echomail message. Leave blank to disable. |
| `areafix_password` | Password **we** send when requesting areas from **our own uplink's** AreaFix — see §8.4. |
| `[fido.areas]` | Maps echomail `AREA:` tags to local conference IDs — see §3. |
| `[[fido.downlinks]]` | Systems that subscribe to our echomail areas via AreaFix — see §8.1. |

> **Address format reminder:** VirtBBS only understands the standard 4D form `zone:net/node[.point]`. There is no separate "domain" field (e.g. `.fidonet.org`) — that's a FidoNet Internet-gateway concept VirtBBS does not implement.

---

## 2. Conferences and echomail flags

Each VirtBBS conference can be linked to a FidoNet echomail area independently
of the `[fido.areas]` map (this is the mechanism used by the **scan** step —
see §5). Configure it via the sysop **[E]cho flags** menu, or through the GUI's
FidoNet → Echo Flags tab, or the `conferences.update` API call:

| Field | Meaning |
|---|---|
| `Echo` | `true` marks this conference as an echomail area (vs. a local-only conference). |
| `EchoTag` | The `AREA:` tag for this conference, e.g. `VIRTBBS_SUPPORT`. Must match the tag your uplink/downlinks use for the same area. |
| `UplinkAddr` | Per-conference uplink override. Leave blank to use the network's default `uplink`. Useful if you receive different echo areas from different systems. |
| `Network` | Which configured network this conference's echomail belongs to. Leave blank (VirtBBS will store it as `"FidoNet"`) for the primary network, or set it to match a `[[fido.networks]] name` (see §6) for additional networks. |

---

## 3. `[fido.areas]` — inbound area routing (toss)

`[fido.areas]` is a simple map from `AREA:` tag to conference ID, used **only
by the toss step** (inbound mail) to decide which conference an incoming
echomail message belongs to:

```toml
[fido.areas]
  FIDO_GENERAL    = 1
  VIRTBBS_SUPPORT = 2
```

- The key is the exact `AREA:` tag as it appears in the inbound packet (case-sensitive, no `AREA:` prefix).
- The value is the numeric conference ID (see conference list in the sysop menu or `conferences.list` API).
- Any inbound echomail whose `AREA:` tag isn't listed here is **skipped** (not imported) and counted in the toss result's `Skipped` total.

> **Why two area mappings?** `[fido.areas]` (toss/inbound) and each conference's `EchoTag` field (scan/outbound) are independent on purpose — a conference can be a *recipient* of an echo area without VirtBBS originating/relaying traffic for it, and vice versa. For a normal two-way echo area, set up both: add the tag to `[fido.areas]` so inbound mail is filed correctly, **and** set the conference's `Echo=true`/`EchoTag` so locally-posted replies get scanned back out.

---

## 4. Tossing (processing inbound mail)

"Tossing" reads every `.pkt` file in `inbound_dir`, imports recognised
messages, and moves processed packets to `<inbound_dir>/.tossed/`.

Ways to trigger a toss:
- **In-BBS:** Sysop menu → FidoNet → `[T]oss inbound`
- **CLI:** `virtbbs -fido-toss`
- **API:** `fido.toss`

What happens during toss:
- **Netmail** (no `AREA:` line) is filed into conference 0 (General), addressed to the recipient named in the message.
- **Echomail** is routed via `[fido.areas]` (§3); unknown areas are skipped.
- Each message's `^AMSGID`, `SEEN-BY:`, and `^APATH` are parsed out and stored as structured metadata (not shown in the message body) so they can be correctly re-emitted if you relay the message onward. The tear line (`--- ...`) and `* Origin: ...` line are **kept visible** in the stored body, matching how real FidoNet readers display them.
- Duplicate packets (same `^AMSGID` re-processed twice, e.g. after a crash) are detected and skipped automatically.
- A netmail with **Subject `PING`** triggers an automatic `PONG` reply — see §7.

---

## 5. Scanning (sending outbound echomail)

"Scanning" exports every not-yet-sent echo-flagged message into outbound
`.pkt` file(s) in `outbound_dir`, bundling multiple conferences addressed to
the same uplink into a single packet. Any AreaFix-subscribed downlinks (§8)
for an area also get their own packet automatically.

Ways to trigger a scan:
- **In-BBS:** Sysop menu → FidoNet → `[S]can outbound`
- **CLI:** `virtbbs -fido-scan`
- **API:** `fido.scan`

What gets added to each outgoing message automatically:
- `AREA:<tag>`, `^AMSGID`, `^ATZUTC` (your local UTC offset, e.g. `+0200`)
- A random line from `taglines_file`, if configured
- A standard tear line (`--- VirtBBS <version>`) and Origin line (`* Origin: <BBS name> (<your address>)`)
- `SEEN-BY:` and `^APATH:` lines — merged with whatever was already present if the message arrived via toss and is being relayed onward, or starting fresh (just your own address) for locally-authored posts

Once a message has been successfully written into a packet, it is marked
internally so it will **not** be re-sent on the next scan — each message is
exported to a given uplink exactly once.

Netmail is sent immediately at compose time (not via the scan step) — see
the `[K]NetMail` option in the Messages menu, or `fido.netmail.send` via the
API.

---

## 6. Polling your uplink (BinkP)

"Polling" connects to your uplink over BinkP, sends any outbound `.pkt`
files, and receives anything waiting for you.

Ways to trigger a poll:
- **In-BBS:** Sysop menu → FidoNet → `[P]oll uplink`
- **API:** `fido.poll` (params: `{"network": "<name>"}`, default network if blank)

Polling does **not** automatically toss what it receives — run `[T]oss
inbound` (or `-fido-toss`) afterward to import newly-received packets.

---

## 7. Multiple networks

VirtBBS can participate in more than one FidoNet-compatible network (e.g.
FidoNet plus a regional/hobby net) at the same time. The top-level `[fido]`
table describes your **primary** network (always named `"FidoNet"`
internally). Add others under `[[fido.networks]]`:

```toml
[fido]
  enabled      = true
  address      = "1:1/1"
  uplink       = "1:1/100"
  inbound_dir  = "fido/inbound"
  outbound_dir = "fido/outbound"
  nodelist_dir = "fido/nodelist"

  [fido.areas]
    FIDO_GENERAL = 1

[[fido.networks]]
  name         = "LovelyNet"
  enabled      = true
  address      = "80:774/1"
  uplink       = "80:774/100"
  password     = ""
  inbound_dir  = "fido/lovelynet/inbound"
  outbound_dir = "fido/lovelynet/outbound"
  nodelist_dir = "fido/lovelynet/nodelist"
  binkp_port   = 24554
  taglines_file = ""

  [fido.networks.areas]
    LOVELY_CHAT = 3
```

Each `[[fido.networks]]` entry is a **fully independent** network: its own
address, uplink, inbound/outbound directories, nodelist, and area map. Use
a distinct `inbound_dir`/`outbound_dir` per network so packets don't collide.

- **Scanning** (§5) iterates every enabled network and writes separate `.pkt` files for each — link a conference to a specific network via its `Network` field (§2) so the scanner knows which network's address/uplink to use for it.
- **Tossing** (§4) currently only processes the **primary** network's `inbound_dir`. If you run additional networks, point your BinkP/mailer setup so each network's inbound mail lands in that network's own `inbound_dir`, and toss each directory separately (e.g. via a small wrapper script calling `virtbbs -fido-toss` with a per-network config, or via the API against each network).
- **Polling** (§6) takes a `network` parameter specifically so you can poll each uplink independently.

---

## 8. AreaFix

VirtBBS implements AreaFix in both directions: **responding** to subscription
requests from your downlinks, and **requesting** areas from your own uplink.

### 8.1 Configuring downlinks (systems that subscribe to your areas)

Add each downlink under `[fido]` (or `[[fido.networks]]` for a non-primary
network):

```toml
[fido]
  ...
  [[fido.downlinks]]
    name     = "Bob's BBS"
    address  = "1:2/4"
    password = "letmein"
```

| Field | Meaning |
|---|---|
| `name` | Display name only, shown in the sysop AreaFix menu. |
| `address` | The downlink's `zone:net/node`. Must match exactly (point ignored) for AreaFix requests from this system to be accepted. |
| `password` | What the downlink must supply as the first non-blank line of its AreaFix netmail. Leave blank to allow unauthenticated requests from this address (not recommended). |

There's no separate config needed for *which* areas a downlink can have —
any area with a matching `EchoTag` (§2) can be requested; VirtBBS validates
the tag against your conferences (or `[fido.areas]` as a fallback) before
accepting a subscription.

### 8.2 How the responder works

A downlink emails `AreaFix` at your address with a netmail body like:

```
letmein
+VIRTBBS_SUPPORT
-OLD_AREA
%QUERY
```

- The **first non-blank line** must match the downlink's configured `password` (or be skipped entirely if the downlink has no password configured).
- `+TAG` subscribes, `-TAG` unsubscribes, `%LIST` lists every area available, `%QUERY` lists current subscriptions, `%HELP` shows command help.
- VirtBBS replies immediately (not via the scan step) with a netmail confirming what changed and the resulting subscription list.
- The original request is also stored as ordinary netmail (conference 0) so the sysop can audit what's been requested.

This all happens automatically during **toss** (§4) — no extra step required.

### 8.3 How fan-out to downlinks works

Once a downlink is subscribed to an area, the **scan** step (§5) automatically
includes them: every time a message is exported for that area, VirtBBS writes
an additional `.pkt` addressed directly to each subscribed downlink, alongside
the normal one addressed to your uplink. No separate uplink override or
per-conference configuration is needed for this — it's purely subscription-
driven.

> **Note:** export tracking (`fido_exported_at`, §5) is per-message, not
> per-destination. If the uplink's packet write succeeds but a downlink's
> packet write fails (e.g. a permissions error), the message is still marked
> exported and will not be retried for that downlink on the next scan. This
> is a known simplification, not a typical real-world failure mode (the same
> directory and write path is used for both).

### 8.4 Requesting areas from your own uplink

If your uplink also runs AreaFix, VirtBBS can act as a downlink of theirs.
Set the password they issued you:

```toml
[fido]
  ...
  areafix_password = "whatever-your-uplink-gave-you"
```

Then, in-BBS: Sysop menu → FidoNet → `[A]reaFix` → `[U]pstream request` —
enter the area tags to subscribe/unsubscribe, space-separated. VirtBBS sends
the request immediately; your uplink's own AreaFix will reply by netmail
once it's processed it.

### 8.5 Sysop menu reference

Sysop menu → FidoNet → `[A]reaFix`:

| Key | Action |
|---|---|
| `[D]` | Add a downlink (name, address, password) — saved to `VirtBBS.DAT`. |
| `[R]` | Remove a downlink by address — also clears its subscriptions. |
| `[U]` | Send an AreaFix subscribe/unsubscribe request to your own uplink. |

The main listing shows each configured downlink alongside its current
subscriptions.

### 8.6 Limitations

- AreaFix admin (add/remove downlinks, view subscriptions) is currently **in-BBS sysop menu only** — not yet exposed through the management API or the .NET GUI.
- Toss (§4) only processes the **primary** network's inbound directory, so the responder currently only handles AreaFix requests arriving on the primary network — same limitation as multi-network toss in general (§7).

---

## 9. PING — netmail connectivity test

VirtBBS implements the long-standing FidoNet "ping" netmail convention (not
an official FTS standard, but widely supported by classic mailers): a
netmail with **Subject `PING`** sent to a node triggers an automatic
**Subject `PONG`** reply, confirming mail flow between two systems.

- **Sending a ping:** Sysop menu → FidoNet → `[I]Ping a node`, then enter the
  destination address (`zone:net/node`). VirtBBS looks up the sysop name from
  your local nodelist if available, builds a `PING` netmail, and routes it
  via your configured uplink immediately (no scan step needed).
- **Receiving a ping:** handled automatically during toss (§4) — any inbound
  netmail with Subject `PING` (matched case-insensitively) gets an immediate
  `PONG` reply queued to your outbound directory, addressed back to the
  sender, reporting the time it was received and the original PING's
  timestamp.
- **No loop risk:** the auto-responder only ever triggers on Subject `PING`
  exactly — it never replies to a `PONG`, so two systems both running the
  auto-responder won't ping-pong forever.

You can also originate a `PING` manually through the ordinary netmail
composer (`[K]NetMail`) by simply typing `PING` as the subject — the
dedicated `[I]Ping a node` menu option just saves a few steps (address
lookup + immediate send).

---

## 10. Quick reference — all `[fido]` fields

```toml
[fido]
  enabled          = false              # master on/off switch
  address          = "1:1/1"            # this BBS's own FidoNet address
  uplink           = ""                 # your boss/hub node's address
  password         = ""                 # shared session/packet password
  inbound_dir      = "fido/inbound"     # where toss reads .pkt files from
  outbound_dir     = "fido/outbound"    # where scan/netmail writes .pkt files to
  nodelist_dir     = "fido/nodelist"    # NODELIST.* files for address lookups
  binkp_port       = 24554              # BinkP port used when polling
  taglines_file    = ""                 # optional taglines, one per line
  areafix_password = ""                 # password WE send to OUR uplink's AreaFix

  [fido.areas]                       # AREA: tag → conference ID (inbound routing)
    TAG_NAME = 1

  [[fido.downlinks]]                 # zero or more systems that subscribe to our areas
    name     = "Bob's BBS"
    address  = "1:2/4"
    password = "letmein"

[[fido.networks]]                    # zero or more additional networks
  name             = "NetworkName"
  enabled          = true
  address          = "..."
  uplink           = "..."
  password         = ""
  inbound_dir      = "..."
  outbound_dir     = "..."
  nodelist_dir     = "..."
  binkp_port       = 24554
  taglines_file    = ""
  areafix_password = ""

  [fido.networks.areas]
    TAG_NAME = 3

  [[fido.networks.downlinks]]
    name     = "..."
    address  = "..."
    password = "..."
```
