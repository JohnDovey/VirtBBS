# VirtBBS — AI assistant notes

Guidance for Claude, Cursor, and other coding agents working in this repository. All project documentation lives in this `docs/` directory.

## Repository location

The repo may live on an external volume (e.g. `/Volumes/JohnDovey/Projects/BBS/VirtBBS`). Toolchains are installed on the **host system**, not on that volume.

### JohnDovey drive environment

Before compiling (especially **Android / Gradle**), read and apply paths from:

```bash
source /Volumes/JohnDovey/source-john-dovey.sh
# or: source ~/source-john-dovey.sh
```

That script sets (when `/Volumes/JohnDovey` is mounted):

| Variable | Path |
|----------|------|
| `ANDROID_HOME` | `/Volumes/JohnDovey/Android/Sdk` |
| `GRADLE_USER_HOME` | `/Volumes/JohnDovey/.gradle` |
| `JAVA_HOME` | `/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home` |

Use these values for VirtAnd builds instead of system `/tmp` or default `~/.gradle` on the boot volume.

### Temporary files

On the JohnDovey setup, use **`/Volumes/JohnDovey/tmp/`** for build artifacts, release staging, and scratch output — **not** `/tmp` on the system drive. Example:

```bash
export RELEASE_DIR="/Volumes/JohnDovey/tmp/virtbbs-release-${VERSION}"
```

## Go server

The BBS server is Go (no cgo). See `BUILDING.md` for full instructions.

```bash
go build ./cmd/virtbbs
./virtbbs -config VirtBBS.DAT
```

Sysop administration is web-only (`internal/web`, `/admin/*`). The only JSON-over-TCP API is `internal/userapi` on port 9998 (default) for the VirtAnd Android client — BBS username/password auth, not sysop credentials.

### Door games → ServiceMonitor

Doors live under `DoorGames/` (MathMaze, AnsiArt, …). The live monitored BBS is
`/Volumes/JohnDovey/Projects/ServiceMonitor/services/VirtBBS/`. After changing a door:

1. Bump the **patch** in that door’s `version.go` (same rule as VirtBBS: every change bumps patch).
2. Rebuild and sync:

```bash
cd DoorGames/MathMaze && GOTOOLCHAIN=local go build -o mathmaze .
cd DoorGames/AnsiArt && GOTOOLCHAIN=local go build -o ansiart .
/Volumes/JohnDovey/Projects/ServiceMonitor/scripts/sync-binaries.sh
```

That installs door binaries under `ServiceMonitor/services/VirtBBS/DoorGames/…`. Keep `[[doors]]` in both local `VirtBBS.DAT` and ServiceMonitor’s `VirtBBS.DAT` when adding doors. Browser image convert: `/ansiart` (shared `pkg/ansiart`).

## Web interface

Browser-based BBS UI and sysop admin served by `internal/web`. Templates and static assets live under `paths.www` (default `www/`). See `www/README.md` for routes and feature checklist.

- **Bootstrap 5** + **jQuery** for responsive layout (collapsible nav on mobile)
- Locales: English, Spanish, Afrikaans (`internal/web/locales/*.json`)
- Design inspiration: [BinktermPHP](https://lovelybits.org/binktermphp)

Default URL: **http://localhost:8081/**

Sysop administration: log in as sysop and use **Admin** in the nav bar (`/admin/*`).

## Android app (VirtAnd) — build like ClonesApp

### Reference project (working template)

Copy Gradle/project structure from:
`/Volumes/JohnDovey/Projects/ClonesApp`

Use the same tooling — do not invent a new Android stack:

- Kotlin 2.0 + Jetpack Compose + Room + KSP
- Gradle **9.5.1** wrapper (`./gradlew`, not system `gradle`)
- AGP **8.13.2**, `compileSdk`/`targetSdk` **35**, `minSdk` **26**, JVM **17**
- Version catalog in `android/VirtAnd/gradle/libs.versions.toml`

### SDK and JDK (macOS, JohnDovey drive)

| Item | Path |
|------|------|
| Android SDK | `/Volumes/JohnDovey/Android/Sdk` |
| Gradle user home | `/Volumes/JohnDovey/.gradle` (`GRADLE_USER_HOME` via `source-john-dovey.sh`) |
| JDK 17 | `/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home` |
| Reference app | `/Volumes/JohnDovey/Projects/ClonesApp` |
| VirtAnd project | `android/VirtAnd/` |

Create `android/VirtAnd/local.properties` (gitignored; copy from `local.properties.example`):

```
sdk.dir=/Volumes/JohnDovey/Android/Sdk
```

If Gradle can't find Java or Kotlin fails to compile:

```bash
source /Volumes/JohnDovey/source-john-dovey.sh
# or manually:
export JAVA_HOME="/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home"
export GRADLE_USER_HOME="/Volumes/JohnDovey/.gradle"
```

### VirtAnd layout

- `android/VirtAnd/core/` — pure Kotlin/JVM (`UserApiClient`, QWK parsing). No Android SDK required.
- `android/VirtAnd/app/` — Android app (Compose UI, Room, WorkManager). Requires SDK.

Server API: `internal/userapi` (BBS username/password auth on each request).

### Build commands

```bash
# Verify Android toolchain (reference project):
cd /Volumes/JohnDovey/Projects/ClonesApp && ./gradlew assembleDebug

# VirtAnd JVM module only (no Android SDK):
cd android/VirtAnd && ./gradlew :core:test

# VirtAnd Android APK:
cd android/VirtAnd && ./gradlew :app:assembleDebug
# or:
cd android/VirtAnd && ./android-build.sh
```

### Scaffolding / extending VirtAnd

When adding Android dependencies or UI, mirror ClonesApp patterns:

- `gradle/libs.versions.toml` for dependency versions
- Compose `MainActivity` + navigation graph
- Room + KSP for local cache
- `android-build.sh` for CLI builds on the JohnDovey drive

## Common mistakes to avoid

- Assuming toolchains are on the same drive as the repo — Go, JDK, and Android SDK are on the system install paths above.
- Using `/tmp` for release builds or Gradle scratch — use `/Volumes/JohnDovey/tmp/` on this machine.
- Skipping `source /Volumes/JohnDovey/source-john-dovey.sh` before Android/Gradle work (sets `GRADLE_USER_HOME` and `ANDROID_HOME`).
- Building `:app` without `local.properties` pointing at the SDK.
- Using JDK 21+ or JDK 8 for VirtAnd/ClonesApp — use JDK 17.
