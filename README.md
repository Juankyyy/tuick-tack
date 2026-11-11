<div align="center">

# Tuick-Tack

**Your countdown, in the terminal and in the tray.**

Presets, free-form minutes, a chime and confetti. Windows 10/11.

<sub>In development · Go + Bubble Tea · GPL-3.0</sub>

</div>

---

## ✨ What it does

| Feature | Details |
|---|---|
| ⏱ **Presets** | 5, 10 and 15 min out of the box. Create your own with a name and duration. |
| ⌨️ **Free-form minutes** | `25`, `12:30`, `90s`, `1h30m`. |
| ⏯ **Full control** | Pause, resume, adjust ±1 min and repeat. |
| 📌 **Always at hand** | The countdown and tray icon keep running when you close the TUI. |
| 🖱️ **Tray** | Start a preset, pause, stop or open the TUI from the icon. |
| 🔔 **At zero** | A 4-second chime, an alert icon and full-screen confetti. |
| 🎨 **Three themes** | Midnight, Ember and Mono. |
| 📦 **Portable** | Two `.exe` files, no installer, no admin. |

## 🖥️ A quick look

```text
╭──────────────────────────────────────────────────────────╮
│  tuick·tack                                 ● 5 min      │
│                                                          │
│                    04:32                                 │
│         ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░              │
│         4:32 remaining · ends at 14:32                   │
│                                                          │
│    [ 5 min ]   [ 10 min ]   [ 15 min ]   [ + ]           │
│    space pause  ·  esc cancel  ·  p presets              │
╰──────────────────────────────────────────────────────────╯
```

## 🚀 Getting started

> **In development.** There are no binaries yet: the first milestone is the systray spike.

**Once v1 is released**, it will be as simple as this:

1. Download `tuick-tack.exe` and `tuick-tack-tray.exe` into the same folder.
2. Open `tuick-tack.exe`.
3. The daemon starts on its own. Use the tray icon; no installer and no admin rights required.

On first launch, SmartScreen may ask for confirmation because the `.exe` is unsigned. This is expected.

**For development** (from M0 onwards):

```powershell
go mod download
go run ./cmd/tuick-tack          # TUI
go run ./cmd/tuick-tack-tray     # daemon; no console in release
.\scripts\build.ps1              # both .exe files in dist/
```

To use the app: Windows 10/11. To build: Go 1.24+. Windows Terminal recommended.

## ⌨️ Controls

| Action | Key |
|---|---|
| Type a duration | digits, `:`, `h`, `m`, `s` |
| Start | `Enter` |
| Select a preset | `←` `→` + `Enter` |
| Pause / resume | `Space` |
| Cancel | `Esc` (confirmation) |
| ±1 minute | `+` / `-` |
| Repeat when finished | `r` |
| Presets | `p` |
| Theme | `t` |
| Sound | `s` |
| Help | `?` |
| Close the TUI | `q` |

Duration formats: `25` = 25 min · `12:30` = 12 min 30 s · `90s` · `1h30m`.

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

## ⚙️ Configuration

- **Presets, theme and sound:** `%APPDATA%\tuick-tack\config.json`
- **Logs and cache:** `%LOCALAPPDATA%\tuick-tack\`
- **Autostart:** only if you enable it (`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`).

Everything is local. No accounts, no cloud, and no background services beyond the daemon.

## 🛠️ Development

```text
cmd/tuick-tack        TUI
cmd/tuick-tack-tray   daemon
internal/             engine, IPC, config, tray, TUI
assets/               chime and icons
```

- Behavior and architecture: [`SPEC.md`](SPEC.md)
- Conventions and workflow: [`AGENTS.md`](AGENTS.md)
- Tests: `go test ./...` · Lint: `go vet ./...` · Build: `.\scripts\build.ps1`

## 📄 License

GPL-3.0. See [`LICENSE`](LICENSE).
