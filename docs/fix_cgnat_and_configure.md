# Fix CGNAT and configure a full FidoNet hub

Guide for running **Larry's Farm BBS** as a **VirtNet** FidoNet hub with inbound BinkP, while also serving the web UI (8081), VirtAnd User API (9998), and LovlyNet (downlink).

**Environment (verified):** Vodacom 5G via Huawei **H155-383** (`192.168.8.1`), public IP `41.13.104.31` with reverse DNS `vc-nat-…-umts.vodacom.co.za` — carrier NAT blocks unsolicited inbound connections.

---

## The blocker: you need a real public IP first

A hub is not “poll my uplink only.” Other systems must dial **your** BinkP port. That requires:

| Requirement | Why |
|-------------|-----|
| Public IPv4 (not CGNAT) | Inbound TCP must reach your router |
| Router port forward → VirtBBS host | `24555` (VirtNet), `24554` (LovlyNet if you want inbound there too) |
| Stable hostname | Vodacom unrestricted = dynamic IP; use DDNS |
| `binkp_host` in nodelist | Downlinks read `IBN:hostname:port` |

**What to get (South Africa, ranked):**

1. **Vodacom unrestricted APN** — ask Vodacom or a business reseller to provision it on your SIM. Dynamic public IP, inbound ports open. You sign their risk waiver.
2. **Fixed LTE** (Afrihost/Axxess on Vodacom/MTN) — often includes public IP without CGNAT; better for a hub.
3. **Fiber** — best long-term if available.

Until one of those is in place, you can run VirtNet as a **config testbed** but not as a live hub that downlinks can poll.

### Vodacom APN summary

| APN | Inbound | Port forwarding |
|-----|---------|-----------------|
| `internet` (default) | Blocked (CGNAT) | Useless |
| `unrestricted` (provisioned) | Allowed | Works on router |

See [Flickswitch: unrestricted APN on Vodacom](https://flickswitch.freshdesk.com/support/solutions/articles/23000017931--unrestricted-apn-on-vodacom-how-does-it-work-).

---

## Current VirtBBS layout

| Network | Role | Address | BinkP port | Notes |
|---------|------|---------|------------|-------|
| **VirtNet** | Hub (`uplink=""`) | `300:1/1` | **24555** | Hub network — needs `binkp_host` |
| **LovlyNet** | Downlink (`uplink=227:1/1`) | `227:1/17` | **24554** | Outbound poll works on CGNAT |
| **FidoNet** `[fido]` | Primary stub | empty | 24554 | Networks live under `[[fido.networks]]` |

### Other VirtBBS ports

| Service | Port |
|---------|------|
| Web UI + admin | 8081 |
| VirtAnd User API | 9998 |
| Telnet | 2323 |
| SSH | 3232 |

---

## Phase 1 — Network (once you have public IP)

### 1. Confirm you are off CGNAT

On the Huawei admin UI (`http://192.168.8.1`), check the mobile WAN IP. It should:

- **Not** be `10.x`, `100.64.x`, or `192.168.x`
- **Match** what [ifconfig.me](https://ifconfig.me) shows from your Mac

### 2. Dynamic DNS

Pick a hostname, e.g. `larrysfarm.duckdns.org` (replace in `VirtBBS.DAT` → VirtNet `binkp_host`).

Point an **A record** at your current public IP; update it whenever the IP changes (DuckDNS client, router DDNS, or a cron script).

### 3. Huawei router port forwards

Forward these to the Mac/PC running VirtBBS (e.g. `192.168.8.19`):

| External port | Internal port | Service |
|---------------|---------------|---------|
| **24555** | 24555 | VirtNet BinkP (hub) — **required** |
| **24554** | 24554 | LovlyNet BinkP (optional inbound) |
| 8081 | 8081 | Web UI (optional) |
| 9998 | 9998 | VirtAnd API (optional) |

BinkP **24555** is critical for the hub.

### 4. Verify inbound reachability

From **outside** your LAN (phone on mobile data, or [canyouseeme.org](https://canyouseeme.org)):

- Port **24555** open → hub can receive polls
- If closed, fix APN/router/firewall before touching VirtBBS further

---

## Phase 2 — VirtBBS hub configuration

See also [Using VirtBBS as a Network Hub](Using-VirtBBS-as-a-Network-Hub.md) and [FidoNet Config.md](FidoNet%20Config.md).

### VirtNet hub (`VirtBBS.DAT`)

Applied in the repo `VirtBBS.DAT` (option A patch):

```toml
[[fido.networks]]
  name = "VirtNet"
  enabled = true
  address = "300:1/1"          # primary node (NC on net 1)
  uplink = ""                  # hub — no uplink
  password = "25d62bca695b"
  binkp_port = 24555
  binkp_host = "larrysfarm.duckdns.org:24555"   # ← replace with your DDNS host
  node_flags = ["IBN", "CM", "ITN", "BEER", "TRACE", "PING"]
  nodelist_url = ""
  akas = ["300:300/0", "300:1/0"]
```

**Before going live:** set `binkp_host` to your real DDNS hostname and port.

| Flag | Meaning |
|------|---------|
| `IBN` | Internet BinkP Node |
| `CM` | Continuous Mail — hub available 24h |
| `ITN` | Internet Telnet |
| `BEER`, `TRACE`, `PING` | Standard VirtBBS defaults |

Apply via **Admin → FidoNet → Network Setup → Node Capabilities**, or edit `VirtBBS.DAT` and restart VirtBBS.

### Register yourself in `fido_members`

Generated hub nodelists come from **members**, not only from config.

1. **Admin → Fido → Join requests** (or member edit)
2. Add/approve yourself: **net 1, node 1, Host = on**
3. **Admin → Fido → Nodelist → Rebuild from members**

Published nodelist should look like:

```text
Zone,300,Larry's Farm BBS,Internet,Sysop,-Unpublished-,33600,CM,IBN:larrysfarm.duckdns.org:24555
Host,1,Larry's Farm BBS,Internet,Sysop,-Unpublished-,33600,IBN:larrysfarm.duckdns.org:24555,CM
,1,Larry's Farm BBS,Internet,Sysop,-Unpublished-,33600,IBN:larrysfarm.duckdns.org:24555,CM
```

### Echomail areas

Existing VirtNet area map:

| Tag | Conference |
|-----|------------|
| `VNET.NODELIST` | 3 |
| `VNET.SUPPORT` | 1 |
| `VNET.NODEFILES` | file area 2 |

For each conference used as echo: **Echo** on, **EchoTag** matches AreaFix tags, **Network** = `VirtNet`.

---

## Phase 3 — Downlinks

For each BBS that polls you:

### 1. Approve as member

**Admin → Fido → Join requests**: net **1**, node **2, 3, …**, Host **off**.

### 2. Add downlink

**Admin → Fido → Downlinks** (or `[[fido.networks.downlinks]]` under VirtNet):

```toml
[[fido.networks.downlinks]]
  name = "Bob's BBS"
  address = "300:1/2"
  password = "shared-downlink-password"
```

### 3. Downlink configuration (on their system)

```toml
address = "300:1/2"
uplink  = "300:1/1"
# Poll: larrysfarm.duckdns.org:24555 with hub password
```

### 4. AreaFix flow

1. Downlink sends netmail to `AreaFix` at your address (`+VNET.SUPPORT`, etc.).
2. You confirm subscription via netmail.
3. **Scan** fans out echomail `.pkt` files per subscribed downlink.
4. Downlink **polls** you on 24555 and receives packets.

---

## Phase 4 — Verify the hub is alive

| Check | Where |
|-------|--------|
| `BinkP listening on :24555` | VirtBBS startup log |
| Inbound session from downlink | `logs/binkp.log`, **Admin → Fido → BinkP Log** |
| AreaFix subscriptions | **Admin → Fido → Downlinks** |
| Echomail fan-out | **Admin → Fido → BinkP Stats** |
| Nodelist published | **Admin → Fido → Nodelist** |
| Remote port open | [canyouseeme.org](https://canyouseeme.org) port 24555 |

Test inbound: have a downlink (or second VirtBBS instance) **Poll uplink** against `your-host:24555` with the shared password. Expect `binkp server: [VirtNet]` in the log.

---

## LovlyNet vs VirtNet

| Network | On CGNAT today | After public IP |
|---------|----------------|-----------------|
| **LovlyNet** (`227:1/17` → `227:1/1`) | Outbound poll works | Same; set `binkp_host` only if `227:1/1` must call you inbound |
| **VirtNet hub** | **Broken** — no inbound | Works once 24555 is forwarded and `binkp_host` is set |

You can remain a LovlyNet downlink on CGNAT while preparing the VirtNet hub. The hub goes live only when the connection has a public IP.

---

## Remote access without public IP (non-FidoNet)

These do **not** replace hub BinkP but help 8081 / 9998:

| Goal | Solution |
|------|----------|
| Web UI from anywhere | Cloudflare Tunnel → `8081` |
| VirtAnd away from home | Tailscale → Mac Tailscale IP, port `9998` |
| Same Wi‑Fi | `192.168.8.19:9998` |

**Will not work for FTN hub:** Tailscale, Cloudflare Tunnel, ngrok — nodelist needs stable `IBN:public-host:port`.

---

## Order of operations

1. Get public IP (unrestricted APN or fixed LTE).
2. Set up DDNS; update `binkp_host` in `VirtBBS.DAT`.
3. Forward **24555** on the Huawei router.
4. Confirm port open from outside.
5. Restart VirtBBS; confirm BinkP listener on 24555.
6. Rebuild nodelist from `fido_members`.
7. Add downlinks and AreaFix subscriptions.
8. Test inbound BinkP poll.

---

## What will not work

- Router port forwarding on default Vodacom `internet` APN
- DMZ / UPnP on CGNAT
- Outbound-only BinkP as a hub (leaf node only)
- Tunnels for nodelist `IBN` advertisement

---

## Related docs

- [Using VirtBBS as a Network Hub](Using-VirtBBS-as-a-Network-Hub.md)
- [FidoNet Config.md](FidoNet%20Config.md)
- [AreaFix FileFix TIC.md](AreaFix%20FileFix%20TIC.md)
- [Installation.md](Installation.md) — default ports