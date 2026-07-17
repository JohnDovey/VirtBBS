# NodeGUI (VirtBBS door)

FidoNet Zone 1 daily nodelist browser for VirtBBS. Bundled under `Doors/`
(non-game doors). Upstream: [JohnDovey/NodeGUI](https://github.com/JohnDovey/NodeGUI).

**Current version:** 0.3.0

Users can browse and filter the nodelist. **Import** (`i` / `o`) and changing
the download source URL (`c`) require **sysop** security (DOOR.SYS level ≥ 100).

## Build

```bash
cd Doors/NodeGUI
GOTOOLCHAIN=local go build -o nodegui .
```

Local testing without a drop file:

```bash
./nodegui -local
```

## VirtBBS.DAT

Add to both the repo `VirtBBS.DAT` and ServiceMonitor’s `services/VirtBBS/VirtBBS.DAT`:

```toml
[[doors]]
  name             = "NodeGUI"
  description      = "Browse FidoNet Zone 1 nodelist"
  cmd              = "Doors/NodeGUI/nodegui"
  work_dir         = "Doors/NodeGUI"
  drop_file        = "DOOR.SYS"
  append_drop_file = true
  min_security     = 10
```

### Deploy to ServiceMonitor

```bash
cd Doors/NodeGUI && GOTOOLCHAIN=local go build -o nodegui .
/Volumes/JohnDovey/Projects/ServiceMonitor/scripts/sync-binaries.sh
```

That copies `nodegui` to `ServiceMonitor/services/VirtBBS/Doors/NodeGUI/`.

## Keyboard

| Key | Action |
|-----|--------|
| ↑ / ↓ · j / k | Select node |
| `i` | Download & import latest Z1DAILY (**sysop**) |
| `o` | Import a local nodelist or `.zip` (**sysop**) |
| `/` · `f` | Filter |
| `esc` | Clear filter |
| `c` | Edit download base URL (**sysop**) |
| `r` | Reload from database |
| `?` | About |
| `q` | Quit |

## License

GNU General Public License v3 — see [LICENSE](LICENSE).
