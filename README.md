<div align="center">

<img src="assets/banner.png" width="740" alt="Tuick-Tack">

**Your countdown, in the terminal and in the tray.**

Presets, free-form durations, a sound pack with volume, and full-screen confetti. Windows 10/11.

[![Latest release](https://img.shields.io/github/v/release/Juankyyy/tuick-tack?style=flat-square&color=2ea043)](https://github.com/Juankyyy/tuick-tack/releases/latest)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-2ea043?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Windows%2010%20%7C%2011-0078D6?style=flat-square)](https://github.com/Juankyyy/tuick-tack/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/dl/)

[Download](#download) · [Features](#features) · [Controls](#controls) · [Tray](#tray) · [Configuration](#configuration) · [Architecture](#architecture) · [Development](#development)

</div>

![Tuick-Tack idle screen with 12:30 typed, the 5 min preset selected and the key hints](assets/screenshot.png)

## Download

[**Download the latest release for Windows**](https://github.com/Juankyyy/tuick-tack/releases/latest), then run `tuick-tack.exe`.

- Portable: a single executable, with no installer, no admin rights and no dependencies.
- Windows 10 or 11, 64-bit. The build is unsigned, so SmartScreen may warn on first launch.
- The executable is both the TUI and the background daemon. Run it and the app takes care of the rest.
- Check the download against `SHA256SUMS.txt` from the same release:

```powershell
Get-FileHash .\tuick-tack.exe -Algorithm SHA256
```

On first launch the TUI starts a hidden daemon and connects to it. Close the TUI and the
countdown, the sound and the tray icon keep running.

## Features

| Feature | What you get |
|---|---|
| **Presets** | 5, 10 and 15 minutes out of the box. Create, edit and delete your own. |
| **Free-form durations** | `25`, `12:30`, `90s`, `1h30m`, `1:02:03` — from one second to 24 hours. |
| **Full control** | Pause, resume, add or subtract a minute, and repeat. |
| **Always running** | The daemon owns the timer, so closing the TUI never stops a countdown. |
| **Tray menu** | Start a preset, pause, stop, change the sound or volume, and open the TUI. |
| **At zero** | The selected sound at the chosen volume, an alert tray icon and full-screen confetti. |
| **Sound pack** | Chime, Marimba, Digital and Deep; 0–100% volume in 5% steps, with preview and mute. |
| **Three themes** | Midnight, Ember and Mono. |
| **Single instance** | Running the executable again focuses the open TUI instead of opening a second one. |
| **Portable** | One executable, no installer, no admin, no cgo. |

## Controls

| Context | Keys |
|---|---|
| Idle | Digits, `:`, `h`, `m`, `s` type a duration; `Backspace` deletes; `Enter` starts |
| Idle | `←` `→` select a preset; `p` presets; `t` theme; `v` sound settings; `s` mute; `?` help; `q` close |
| Running | `Space` pause; `Esc` confirm cancel; `+` / `-` ±1 min; `v` sound settings; `?` help; `q` close |
| Paused | `Space` resume; `Esc` confirm cancel; `+` / `-` ±1 min; `v` sound settings; `?` help |
| Finished | `r` repeat; `v` sound settings; `Enter` / `Esc` close the celebration |
| Sound panel | `↑` `↓` select the sound; `←` `→` volume ±5%; `Space` preview; `m` mute; `Esc` close |
| Presets panel | `↑` `↓` move; `Tab` actions; `←` `→` choose an action; `Enter` start or run; `a` add; `e` edit; `d` delete; `r` restore defaults; `Esc` close |
| Form | `Tab` next field; `Enter` save; `Esc` cancel |

In the app, key hints use the `[key] action` format, for example `[Space] pause`.

Duration formats: `25` = 25 minutes · `12:30` = 12 min 30 s · `90s` · `1h30m` · `1:02:03`.
The accepted range is one second to 24 hours.

> In idle, `s` adds a seconds unit when the input already contains a number (`90s`);
> `Shift+S`, or `s` on an empty input, toggles sound. `v` opens the sound panel from any screen.

## Tray

Left click opens the TUI, or brings the open one to the front. Right click opens the menu.

```text
Tuick-Tack · 04:32 remaining
─────────────────────────────
Open Tuick-Tack
Start            ▸ 5 min · 10 min · 15 min · Custom…
Pause / Resume
Stop
Sound            ✓
Sound settings   ▸ Chime · Marimba · Digital · Deep · Preview
Volume 80%       ▸ -5% · +5% · 0% · 25% · 50% · 75% · 100%
Start with Windows
─────────────────────────────
Quit
```

The tray owns the timer. Closing the TUI leaves the countdown and the selected sound alive,
and **Quit** stops everything. A daemon restart loses the active countdown; persistence is
future work.

## Configuration

- **Presets, theme and sound settings:** `%APPDATA%\tuick-tack\config.json`
- **Logs and sound cache:** `%LOCALAPPDATA%\tuick-tack\` (`sounds\`)
- **Autostart:** only if you enable it — `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
  value `TuickTack`, pointing to `tuick-tack.exe --daemon --hidden`. No admin rights.

```json
{
  "version": 2,
  "theme": "midnight",
  "sound": true,
  "sound_name": "digital",
  "volume": 80,
  "presets": [
    { "id": "p5",  "label": "5 min",  "seconds": 300 },
    { "id": "p10", "label": "10 min", "seconds": 600 },
    { "id": "p15", "label": "15 min", "seconds": 900 }
  ]
}
```

The daemon is the only writer and writes atomically. A corrupt config is backed up as
`config.bad-<timestamp>.json` and replaced with defaults. A version 1 config is migrated to
version 2 on first load with Digital at 80%.

### CLI flags

```text
tuick-tack --version
tuick-tack --install-startup
tuick-tack --uninstall-startup
tuick-tack --daemon            # background daemon with a console (development)
tuick-tack --daemon --hidden   # daemon without a console (used by autostart)
```

## Architecture

```text
        one executable, two processes (both are tuick-tack.exe)
┌────────────────────────────┐  named pipe  ┌────────────────────────────────────┐
│ TUI (no arguments)         │◄────JSON────►│ daemon (--daemon)                  │
│ Bubble Tea v2              │    10 Hz     │ systray on the main goroutine      │
│ canvas + bigfont + fx      │              │ timer engine · IPC · config        │
│ keyboard, mouse, modals    │              │ PlaySoundW · PCM gain · autostart  │
└────────────────────────────┘              └────────────────────────────────────┘
```

- The daemon is the single source of truth. The TUI renders snapshots and interpolates
  locally for smoothness; it never keeps timer truth of its own.
- One named pipe per user and session; one daemon per session (named mutex).
- The pipe protocol is versioned. The TUI reconnects every second and disables commands
  while offline.
- On finish the daemon increments `finish_seq` once, plays the selected sound at the
  configured volume and updates the tray; the TUI celebrates when it sees the new sequence.

```text
cmd/tuick-tack        single entrypoint: TUI and daemon (--daemon)
internal/             engine, IPC, config, daemon, tray, TUI
assets/               sounds, icons, banner and README screenshot
tools/                sound, icon and banner generators
winres/               executable icon and version resources (.syso)
scripts/build.ps1     portable release build
```

## Development

Requires Windows 10/11, Go 1.26+ and PowerShell 7. CGO is disabled, so no C compiler is needed.

```powershell
go mod download                  # install dependencies
go run ./cmd/tuick-tack          # TUI (starts the hidden daemon)
go run ./cmd/tuick-tack --daemon # daemon in the foreground
go test ./...                    # tests
go vet ./...                     # lint
go generate ./...                # regenerate sounds, icons, banner and .syso
```

### Build from source

```powershell
.\scripts\build.ps1              # dev build into dist/
.\scripts\build.ps1 -Version v0.1.2
```

The script regenerates the assets, checks formatting, runs vet and the tests, and writes:

```text
dist/
  tuick-tack.exe
  SHA256SUMS.txt
```

Run `dist\tuick-tack.exe`. If the daemon is not running, the TUI starts a hidden copy of
itself with `--daemon` and connects. One file; there is no installer.

- Behavior and full design: [`SPEC.md`](SPEC.md)
- Conventions and workflow: [`AGENTS.md`](AGENTS.md)

## Troubleshooting

| Symptom | What to check |
|---|---|
| TUI cannot connect | The TUI starts the daemon itself; check `%LOCALAPPDATA%\tuick-tack\logs\tuick-tack.log`. |
| No sound | Check the Windows volume and output device, that **Sound** is checked in the tray, and that the selected sound is not at 0%. The countdown still finishes visually. |
| Tray icon missing | Windows hides new tray icons; open the overflow chevron. |
| Very small window | Below 48×22 the TUI switches to a plain-text compact view (clock, label, state and essential keys); the styled layout returns above that size. |
| Presets disappeared | A corrupt config is backed up and defaults are restored. Look for `config.bad-*.json` in `%APPDATA%\tuick-tack`. |
| SmartScreen warning | Expected for an unsigned executable. |
| Timer lost after a crash | The timer lives in memory; restart the daemon and start again. Persistence is out of scope for this release. |

## License

GPL-3.0. See [`LICENSE`](LICENSE).
