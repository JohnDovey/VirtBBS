# MathMaze

BBS door game for VirtBBS: navigate a random maze with arrow keys.
Each unsolved room is locked until you solve a maths question by pressing **1–4**.

Current version: **1.0.2** (see `version.go`).

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
- bulletin at `bulletin_path` (default `../../display/MATHMAZE.ANS`)
- `MATHMAZE.ANS` in this directory is the committed starter / reference Top Scores bulletin

## Controls

| Key | Action |
|-----|--------|
| Arrow keys | Move through unlocked rooms |
| 1–4 | Choose an answer when a room is locked |
| Q | Quit and save scores |

## Scoring

- Correct answer: **+5** (enter the room)
- Wrong answer: **-2** (level score floored at 0); new question with new choices
- Questions get harder each level
