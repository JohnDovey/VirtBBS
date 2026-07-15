# VirtAnd

A Kotlin/Android "point" client for VirtBBS, modeled on classic FidoNet
point software for VirtBBS. Runs mostly
offline: syncs new mail via the User API `messages.sync`, lets the user browse a
previously-synced file catalog and queue downloads/uploads, and only talks
to the network during an explicit "synchronize" (manual, or a best-effort
WorkManager background pass).

## Module layout

```
android/VirtAnd/
├── core/   — pure Kotlin/JVM: UserApiClient, ANSI SGR parser.
│             No Android dependency at all — actually compiled and
│             test-verified in the development environment (see below).
└── app/    — the real Android app: Room (local cache), WorkManager
              (background sync), Compose UI. Requires the Android SDK to
              even configure — NOT verified in the development environment
              (no Android SDK was available there). See "Verification
              status" below.
```

This split exists specifically so the substantial, non-trivial parts of
VirtAnd's logic — the wire protocol and ANSI parsing — could be genuinely
compiled and unit-tested here, rather than written blind the way the whole
project would otherwise have to be.

## What it does

- **`core/UserApiClient.kt`** — JSON-over-TCP client for
  `internal/userapi`: one fresh socket per call, newline-delimited
  JSON request/response, BBS username/password auth.
- **`core/AnsiParser.kt`** — minimal SGR ANSI → styled spans (foreground
  30–37/90–97, background 40–47, bold, reset); unsupported CSI stripped.
- **`app/sync/SyncEngine.kt`** — the single "synchronize" entry point:
  conference list refresh → `messages.sync` import → file catalog refresh →
  execute queued downloads → execute queued uploads → nodelist
  version-check per subscribed network → upload queued replies via
  `messages.post` (queue is only cleared on confirmed success).
- **`app/sync/SyncWorker.kt`** — periodic background sync via WorkManager.
  Per the plan, this is accepted-as-is best-effort: Doze/battery
  restrictions mean it won't run promptly (15-minute minimum interval,
  plus further OS deferral) — the primary flow is the user tapping
  Synchronize manually.
- **`app/ui/`** — Compose UI with four tabs (Messages, Files, Queue,
  Settings), message detail view with ANSI rendering, offline message/file
  search, offline compose/reply, file search (local cache and server),
  upload description prompt, queue management, connection test, local
  new-mail notifications after sync, and FidoNet node lookup. The app bar
  shows `session.whoami` after sync.

## Building

Gradle wrapper and version catalog match
[ClonesApp](/Volumes/JohnDovey/Projects/ClonesApp) (Gradle 9.5.1, AGP
8.13.2, Kotlin 2.0, JDK 17). See `../../docs/CLAUDE.md` for SDK paths and
AI-assistant notes.

Copy `local.properties.example` to `local.properties` if you don't already
have one:

```bash
sdk.dir=/Volumes/JohnDovey/Android/Sdk
```

```bash
cd android/VirtAnd
export JAVA_HOME="/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home"

./gradlew :core:test           # pure JVM — no Android SDK needed
./gradlew :app:assembleDebug   # needs Android SDK + local.properties
./android-build.sh               # same as :app:assembleDebug
```

Open in Android Studio: **File → Open →** `android/VirtAnd`

> **JDK note:** use JDK 17 explicitly. Newer JDKs can break older Kotlin
> toolchains or trigger `Internal compiler error` from `compileKotlin`.

## Verification status

**`:core` was actually compiled and tested** in the development
environment (Gradle 9.5.1 + JDK 17, no Android SDK present) —
`gradle :core:test` passes `UserApiClientTest` and `AnsiParserTest`.
This caught and fixed real bugs before they shipped (e.g. ISO-8859-1 body
decoding in the original QWK parser work).

**`:app` now builds** with the Android SDK on the JohnDovey drive
(`./gradlew :app:assembleDebug`, Gradle 9.5.1 + AGP 8.13.2 + Kotlin 2.0,
matching ClonesApp). It was originally written without an SDK available;
manual review before the first real compile caught two bugs:
- `executeQueuedDownloads` originally called `files.download` and threw
  the response away without ever saving the file — fixed to decode the
  base64 payload and write it to app-specific external storage.
- The upload file picker's `OpenDocument` URI grant is temporary by
  default; since uploads are only executed on the *next* synchronize
  (possibly after the app/device has restarted), the read permission
  needed to be persisted at pick-time via
  `takePersistableUriPermission` — fixed.

The first real `:app:assembleDebug` also needed two small compile fixes
(missing `kotlinx.serialization.json.int`/`long` imports in `SyncEngine.kt`,
`@OptIn(ExperimentalMaterial3Api::class)` for `TopAppBar` in `MainActivity.kt`).
Runtime behaviour on a device/emulator still needs manual verification.

## Known limitations

- WorkManager background sync is best-effort only (15-minute minimum
  interval, plus OS deferral) — manual **Synchronize** is the primary flow.
- Local notifications require `POST_NOTIFICATIONS` on Android 13+; denied
  permission means no alerts (sync still works).
- ANSI rendering covers common SGR colors/bold only — no full CP437 glyph
  set or advanced terminal features.
- FidoNet node lookup still requires network access (not cached offline).
- Server file search remains available when online; local search covers the
  synced Room cache only.
