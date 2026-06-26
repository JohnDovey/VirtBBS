# VirtTerm

A graphical .NET 8 / WinForms terminal client for VirtBBS, connecting over
VirtBBS's own TLS protocol (`internal/virtterm` on the server, default port
**6323**) instead of Telnet or SSH — Phase 3 of the VirtAnd/VirtTerm plan.

## What it does

- Renders an 80x25 CP437/ANSI character grid (`Terminal/AnsiScreen.cs` +
  `Terminal/TerminalControl.cs`), fed raw bytes from the TLS connection —
  the exact same byte stream a Telnet client would see, since the server
  hands `internal/virtterm` connections straight to the unmodified
  `session.Run()`.
- A native Windows `MenuStrip`, built client-side from a small static table
  mirroring `mainMenu()`'s fixed items (`Menu/DynamicMenuBuilder.cs`).
  Clicking a top-level item sends that single keystroke into the terminal
  connection — nothing more. Multi-step flows (composing a message,
  transferring a file) are **not** modeled natively; they stay manual
  typing in the terminal pane. Every generated item is enabled only while
  the terminal is showing VirtBBS's literal `"Command: "` prompt, to avoid
  injecting a keystroke into the wrong field mid-flow.
- Per-device API token login (`Net/UserApiClient.cs`) against
  `internal/userapi` — generate a token on the BBS side via the profile
  menu's **[T]okens** option before connecting here.
- Nodelist "has it changed" polling (`Nodelist/NodelistSyncService.cs`)
  against `fido.nodelist.version`, once per connection, for whichever
  networks are listed in Settings.

## Building

Requires the [.NET 8 SDK](https://dotnet.microsoft.com/download/dotnet/8.0)
on Windows (WinForms is Windows-only).

```powershell
cd dotnet-virtterm\VirtTerm
dotnet build
dotnet run
```

> **Note:** this project was written and reviewed without access to a
> Windows machine or the .NET SDK in the development sandbox — there was no
> way to `dotnet build`/`dotnet run` it here. The code has been written
> carefully against the same patterns already proven in
> `gui-dotnet/VirtBBS.GUI` (which *was* built successfully in this repo),
> but **it has not actually been compiled or run**. Build it on a real
> Windows + .NET 8 machine before relying on it, and expect to fix the
> usual first-build nits (a missed `using`, a property name typo) that
> only a real compiler catches.

## Fonts

For pixel-accurate CP437 box-drawing/block art, install a real DOS-VGA
font such as **Px437 IBM VGA8** or **Perfect DOS VGA 437** — see
`Terminal/TerminalControl.PreferredFontFamilies`. Without one, it falls
back to Consolas, which renders box-drawing characters reasonably but not
identically to a real DOS font.

## Known limitations (by design, see the plan)

- Fixed 80x25 grid — no resize negotiation. VirtBBS's own session layer is
  hard-baked to this size, so there's nothing to negotiate.
- No native UI for composing messages, browsing files, or any other
  multi-step BBS flow — those are typed directly into the terminal pane.
- No "who am I" endpoint exists in `internal/userapi` yet, so whether the
  Sysop Menu item is shown is just a checkbox in Settings
  (`AppSettings.IsSysop`), not a real security check — the BBS itself still
  enforces the actual security-level gate if a non-sysop sends `S` anyway.
- The server's TLS certificate is self-signed with no CA, so
  `TerminalConnection` accepts any certificate (same trust-on-first-connect
  model as SSH host keys) rather than validating against a certificate
  chain.
