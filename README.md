<div align="center">

# Tuick-Tack

**Your countdown, in the terminal and in the tray.**

Presets, free-form minutes, a chime and confetti. Windows 10/11.

<sub>v1 feature-complete · Go + Bubble Tea · GPL-3.0</sub>

</div>

---

## ✨ What it does

| Feature | Details |
|---|---|
| ⏱ **Presets** | 5, 10 and 15 min out of the box. Create your own with a name and duration. |
| ⌨️ **Free-form minutes** | `25`, `12:30`, `90s`, `1h30m`. |
| ⏯ **Full control** | Pause, resume, adjust ±1 min and repeat. |
| 📌 **Always at hand** | The countdown, chime and tray icon keep running when you close the TUI. |
| 🖱️ **Tray** | Start a preset, pause, stop or open the TUI from the icon. |
| 🔔 **At zero** | A 4-second chime, an alert icon and full-screen confetti. |
| 🎨 **Three themes** | Midnight, Ember and Mono. |
| 📦 **Portable** | Two `.exe` files, no installer, no admin, no cgo. |

## 🖥️ A quick look

```text
   ╭────────────────────────────────────────────────────────────────────────╮
   │ ● tuick·tack                                                  running  │
   │                                                                        │
   │                      ███     █        ████   ███                       │
   │                     █   █   ██    █       █ █   █                      │
   │                     █  ██  █ █    █       █     █                      │
   │                     █ █ █ █  █         ███     █                       │
   │                     ██  █ █████   █       █   █                        │
   │                     █   █    █    █       █  █                         │
   │                      ███     █        ████  █████                      │
   │                                 5 min                                  │
   │    ██████░░░░░░░░░┆░░░░░░░░░░░░░░░┆░░░░░░░░░░░░░░░┆░░░░░░░░░░░░░░░░    │
   │                    04:32 remaining · ends at 14:04                     │
   │                                                                        │
   │             [ pause ]  [ +1 min ]  [ -1 min ]  [ cancel ]              │
   │        space pause · esc cancel · + / - 1 min · ? help · q quit        │
   ╰────────────────────────────────────────────────────────────────────────╯
```

## 🚀 Getting started

### Build from source

Requires **Go 1.26+** on Windows 10/11. No C compiler is needed (`CGO_ENABLED=0`).

```powershell
go mod download
.\scripts\build.ps1              # dev build into dist/
.\scripts\build.ps1 -Version v1.0.0
```

The build script regenerates the chime/icons, checks formatting, runs vet and
the tests, and produces:

```text
dist/
  tuick-tack.exe         console TUI
  tuick-tack-tray.exe    windowsgui daemon (no console)
  SHA256SUMS.txt
```

Run `dist\tuick-tack.exe`. If the daemon is not running, the TUI starts
`tuick-tack-tray.exe` from the same folder and connects. Keep both `.exe`
files together; there is no installer.

### Development

```powershell
go run ./cmd/tuick-tack-tray    # daemon in a console
go run ./cmd/tuick-tack         # TUI
go test ./...                   # tests
go vet ./...                    # lint
go generate ./...               # regenerate chime, icons and .syso
```

On first launch SmartScreen may warn because the executable is unsigned. That
is expected for v1.

## ⌨️ Controls

| Context | Key |
|---|---|
| Idle | digits, `:`, `h`, `m`, `s` type a duration; `Backspace` deletes; `Enter` starts |
| Idle | `←` `→` select a preset; `p` presets; `t` theme; `s` sound; `?` help; `q` close the TUI |
| Running | `Space` pause; `Esc` confirm cancel; `+` / `-` ±1 min; `?` help; `q` close |
| Paused | `Space` resume; `Esc` confirm cancel; `+` / `-` ±1 min; `?` help |
| Finished | `r` repeat; `Enter` / `Esc` close the celebration |
| Presets | `↑` `↓` move; `a` add; `e` edit; `d` delete; `r` restore defaults; `Enter` start; `Esc` close |
| Form | `Tab` field; `Enter` save; `Esc` cancel |

Duration formats: `25` = 25 min · `12:30` = 12 min 30 s · `90s` · `1h30m` ·
`1:02:03`. The accepted range is 1 second to 24 hours.

> In idle, `s` adds a seconds unit when the input already contains a number
> (`90s`); `Shift+S` or `s` on an empty input toggles sound.

## 🖱️ Tray

Left click: opens the TUI. Right click: menu.

```text
Tuick-Tack · 04:32 remaining
─────────────────────────────
Open Tuick-Tack
Start            ▸ 5 min · 10 min · 15 min · Custom…
Pause / Resume
Stop
Sound            ✓
Start with Windows
─────────────────────────────
Quit
```

The tray owns the timer. Closing the TUI leaves the countdown and chime alive.
Clicking **Quit** stops everything. A daemon restart loses the active countdown;
persistence is future work.

## ⚙️ Configuration

- **Presets, theme and sound:** `%APPDATA%\tuick-tack\config.json`
- **Logs and chime cache:** `%LOCALAPPDATA%\tuick-tack\`
- **Autostart:** only if you enable it — `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
  value `TuickTack`, pointing to `tuick-tack-tray.exe`. No admin rights.

```json
{
  "version": 1,
  "theme": "midnight",
  "sound": true,
  "presets": [
    { "id": "p5",  "label": "5 min",  "seconds": 300 },
    { "id": "p10", "label": "10 min", "seconds": 600 },
    { "id": "p15", "label": "15 min", "seconds": 900 }
  ]
}
```

The daemon is the only writer and writes atomically. A corrupt config is backed
up as `config.bad-<timestamp>.json` and replaced with defaults.

### CLI flags

```text
tuick-tack --version
tuick-tack --install-startup
tuick-tack --uninstall-startup
tuick-tack --daemon            # development fallback, no separate tray exe
```

## 🏗️ Architecture

```text
tuick-tack.exe (console)                 tuick-tack-tray.exe (windowsgui)
┌───────────────────────────┐   named   ┌────────────────────────────────────┐
│ Bubble Tea v2             │◄──pipe───►│ systray on the main goroutine      │
│ canvas + bigfont + fx     │   JSON    │ timer engine · IPC · config        │
│ keyboard, mouse, modals   │  10 Hz    │ PlaySoundW · icons · autostart     │
└───────────────────────────┘           └────────────────────────────────────┘
```

- The daemon is the single source of truth. The TUI renders snapshots and
  interpolates locally for smoothness; it never keeps timer truth of its own.
- One named pipe per user and session; one daemon per session (named mutex).
- The pipe protocol is versioned. The TUI reconnects every second and disables
  commands while offline.
- On finish the daemon increments `finish_seq` once, plays the chime and updates
  the tray; the TUI celebrates when it sees the new sequence.

```text
cmd/tuick-tack        TUI
cmd/tuick-tack-tray   daemon
internal/             engine, IPC, config, daemon, tray, TUI
assets/               chime, icons and embedded asset helpers
tools/                chime and icon generators
winres/               executable icon/version resources (.syso)
scripts/build.ps1     portable release build
```

- Behavior and full design: [`SPEC.md`](SPEC.md)
- Conventions and workflow: [`AGENTS.md`](AGENTS.md)
- Tests: `go test ./...` · Lint: `go vet ./...` · Build: `.\scripts\build.ps1`

## 🧰 Troubleshooting

| Symptom | What to check |
|---|---|
| TUI cannot connect | Is `tuick-tack-tray.exe` next to `tuick-tack.exe`? Check `%LOCALAPPDATA%\tuick-tack\logs\tuick-tack.log`. |
| No sound | Check the Windows volume/sound device and that Sound is checked in the tray. The countdown still finishes visually. |
| Tray icon missing | Windows hides new tray icons; open the overflow chevron. |
| Presets disappeared | A corrupt config is backed up and defaults are restored. Look for `config.bad-*.json` in `%APPDATA%\tuick-tack`. |
| SmartScreen warning | Expected for an unsigned v1 executable. |
| Timer lost after a crash | The timer is in memory; restart the daemon and start again. Persistence is not in v1. |

## 📄 License

GPL-3.0. See [`LICENSE`](LICENSE).
