# MathMaze

BBS door game for VirtBBS: navigate a random maze with arrow keys.
Each unsolved room is sealed by maths gates — walk into the exit labeled with the correct answer.

Current version: **1.0.0** (see `version.go`).

## Versioning

| Component | Rule |
|-----------|------|
| Patch (x.x.**N**) | Bumped on every change or fix |
| Minor (x.**N**.0) | Bumped on significant feature additions |
| Major (**N**.0.0) | Bumped on explicit request |

`./mathmaze -version` prints the current version.

## Build

```bash
cd DoorGames/MathMaze
go build -o mathmaze .
```

## Local test

```bash
./mathmaze -local
```

Uses your terminal; type a name, play with arrow keys, `Q` to quit.

## VirtBBS setup

Add to `VirtBBS.DAT` (local repo and ServiceMonitor’s `services/VirtBBS/VirtBBS.DAT`):

```toml
[[doors]]
  name             = "MathMaze"
  description      = "Solve maths gates to escape the maze"
  cmd              = "DoorGames/MathMaze/mathmaze"
  work_dir         = "DoorGames/MathMaze"
  drop_file        = "DOOR.SYS"
  append_drop_file = true
  min_security     = 10
```

Paths are relative to the BBS working directory (where you run `virtbbs`).

### Deploy to ServiceMonitor

After changing this door, rebuild and sync:

```bash
GOTOOLCHAIN=local go build -o mathmaze .
/Volumes/JohnDovey/Projects/ServiceMonitor/scripts/sync-binaries.sh
```

That copies `mathmaze` to `ServiceMonitor/services/VirtBBS/DoorGames/MathMaze/`.

Copy `config.toml.example` to `config.toml` and set `bulletin_path` to your display directory if needed. On quit, MathMaze writes:

- `scores.json` — persistent high scores
- `MATHMAZE.ANS` — Top Scores bulletin (overall, per level, highest level, lowest per level)

## Controls

| Key | Action |
|-----|--------|
| Arrow keys | Move / choose a gate |
| Q | Quit and save scores |

## Scoring

- Correct gate: **+5**
- Wrong gate: **-2** (level score floored at 0); new question; gates re-labeled
- Questions get harder each level
