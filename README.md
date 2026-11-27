<div align="center">

<img src="assets/banner.png" width="740" alt="Tuick-Tack">

**Your countdown, in the terminal, in the tray and on the taskbar.**

Presets, free-form durations, seven ambient backgrounds, a sound pack with volume, and full-screen pixel-art confetti. Windows 10/11.

[![Latest release](https://img.shields.io/github/v/release/Juankyyy/tuick-tack?style=flat-square&color=2ea043)](https://github.com/Juankyyy/tuick-tack/releases/latest)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-2ea043?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Windows%2010%20%7C%2011-0078D6?style=flat-square)](https://github.com/Juankyyy/tuick-tack/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/dl/)

[Download](#download) · [Features](#features) · [Controls](#controls) · [Tray](#tray) · [Configuration](#configuration) · [Architecture](#architecture) · [Development](#development)

</div>

![Tuick-Tack idle screen with 12:30 typed, the 5 min preset selected and the key hints](assets/screenshot.png)

## Download

[**Download the latest release**](https://github.com/Juankyyy/tuick-tack/releases/latest). Windows has two ways to use the same app:

- **Windows installer (recommended):** run `tuick-tack-setup.exe`. It installs for your user in
  `Documents\tuick-tack`, adds a Start Menu shortcut and registers an uninstaller in
  Windows Settings. No administrator rights.
- **Windows portable:** run `tuick-tack.exe` directly. A single file, no installer and no
  dependencies.

Both Windows copies share `%APPDATA%\tuick-tack\config.json`, so presets, theme, background, sound, volume
and the remembered window placement are the same if you switch between them. If an older Tuick-Tack is running, quit it from the
tray before running the setup for the first time. Windows 10 or 11, 64-bit. The build is
unsigned, so SmartScreen may warn on first launch. Check the download against `SHA256SUMS.txt`:

```powershell
Get-FileHash .\tuick-tack.exe -Algorithm SHA256
Get-FileHash .\tuick-tack-setup.exe -Algorithm SHA256
```

On first launch the TUI starts a hidden daemon and connects to it. Close the TUI and the
countdown, the sound and the tray icon keep running.

## macOS (experimental, frozen)

The macOS port is frozen and not part of the normal release flow. There is no current macOS
release: `v0.3.2` was a one-off dev preview. The **macOS** GitHub Actions workflow stays in the
repository for a manual build, but it no longer runs on push and Windows releases do not include
macOS assets. Dispatch it by hand only when a macOS build or release is explicitly requested.

When a build is made, the workflow produces a universal `tuick-tack-macos-universal.zip` (Apple
silicon and Intel, macOS 13+) that contains `Tuick-Tack.app` and a `README.txt`; dispatching it
with a tag attaches the zip, with its `.sha256`, to an existing GitHub release. The notes below
describe the frozen port; Windows is the supported platform.

The build is ad-hoc signed and not notarized, so Gatekeeper blocks the first launch. After
unzipping, right-click `Tuick-Tack.app` and choose **Open**, or remove the quarantine flag:

```bash
xattr -dr com.apple.quarantine Tuick-Tack.app
```

Move it to `~/Applications` (recommended, no admin) or `/Applications`, and keep it in a
stable location: the login item points at the app. Settings and statistics live in
`~/Library/Application Support/tuick-tack`, cache and logs in
`~/Library/Caches/tuick-tack`, and the optional login item in
`~/Library/LaunchAgents/com.juankyyy.tuick-tack.plist`. The app stays in the menu bar, not in
the Dock; click the icon and choose **Open Tuick-Tack** to start the TUI. **Start at login** toggles the
LaunchAgent. The TUI can check the release and download the new universal zip; it verifies the
SHA-256, reveals the file in Finder and leaves the current app running, so quit the menu bar
app, unzip it and replace `Tuick-Tack.app`. There is no installer and no silent self-update.

| Platform note | Windows | macOS |
|---|---|---|
| TUI, daemon, presets, themes, backgrounds, sounds, statistics | Yes | Yes |
| Menu | System tray (left click opens the TUI) | Menu bar (the click opens the menu; **Open Tuick-Tack** starts the TUI) |
| Autostart | `HKCU\...\Run` | LaunchAgent |
| Remembered terminal placement | Yes | No |
| Installer | Inno Setup, per user | None; move the `.app` |
| Update | Setup or portable `.exe` | Universal zip, revealed in Finder |
| Signing | Unsigned (SmartScreen) | Ad-hoc (Gatekeeper on manual downloads) |

The frozen port receives no new features or fixes. A future macOS release would need its own
validation and QA, and would be announced separately from the Windows releases.

## Features

| Feature | What you get |
|---|---|
| **Presets** | 5, 10 and 15 minutes out of the box. Create, edit and delete your own. |
| **Free-form durations** | `25`, `12:30`, `90s`, `1h30m`, `1:02:03` — from one second to 24 hours. |
| **Full control** | Pause, resume, add or subtract a minute, and repeat. |
| **Always running** | The daemon owns the timer, so closing the TUI never stops a countdown. |
| **Tray** | Start a preset, pause, stop, change the sound or volume, and open the TUI from the Windows system tray. |
| **Mini timer** | Keep the remaining time and a progress bar over the left side of the Windows taskbar while a countdown runs; toggle it from the tray. |
| **Statistics** | `s` opens a full-screen view with a range summary and recent daily focus, a 26-week heatmap, time by start hour and weekday, per-preset totals and the latest sessions. |
| **At zero** | The selected sound at the chosen volume, an alert tray icon and full-screen pixel-art confetti. |
| **Sound pack** | Chime, Marimba, Digital and Deep; 0–100% volume in 5% steps, with preview and mute. |
| **Three themes** | Midnight, Ember and Mono. |
| **Seven backgrounds** | Aurora (the original), Stars, Fireflies, Rain, Tide, Sunset and none. Press `b` to cycle; the daemon remembers the choice. |
| **Readable `none`** | With `none`, the text that floats over the terminal gets a tight black backing, so it stays legible on white, acrylic or a background image. Press `n` to turn the bands off; they only exist with `none`. |
| **Remembers the window placement** | Move or resize the terminal and the next TUI opens at the same screen position with the same columns and rows (Windows only). |
| **Single instance** | Running the executable again focuses the open TUI; it never duplicates the daemon. |
| **Installer (Windows)** | Per-user install in `Documents\tuick-tack`, Start Menu shortcut, uninstaller; no admin. |
| **Portable** | One executable, no installer, no admin, no cgo. |
| **In-app updates** | Check with `u`; installs with the setup or downloads the portable `.exe`. |
| **Version in the header** | The running TUI version follows the wordmark in a quiet tone. |

## Controls

| Context | Keys |
|---|---|
| Idle | Digits, `:`, `h`, `m`, `s` type a duration; `Backspace` deletes; `Enter` starts |
| Idle | `←` `→` select a preset; `p` presets; `t` theme; `b` background; `n` readable bands; `v` sound settings; `m` mute; `s` statistics; `M` mini timer; `u` check/install update; `?` help; `q` close |
| Running | `Space` pause; `Esc` confirm cancel; `+` / `-` ±1 min; `b` background; `n` readable bands; `v` sound settings; `m` mute; `s` statistics; `M` mini timer; `?` help; `q` close |
| Paused | `Space` resume; `Esc` confirm cancel; `+` / `-` ±1 min; `b` background; `n` readable bands; `v` sound settings; `m` mute; `s` statistics; `M` mini timer; `?` help |
| Finished | `r` repeat; `b` background; `n` readable bands; `v` sound settings; `m` mute; `s` statistics; `M` mini timer; `Enter` / `Esc` close the celebration (it also closes itself after 10 s) |
| Statistics | `←` `→` or `Tab` switch section; `1`–`4` jump; `t` range in the overview (7/30/90 days or all time); `↑` `↓` `PgUp` `PgDn` `Home` `End` or the wheel scroll; `Esc` or `s` close |
| Sound panel | `↑` `↓` select the sound; `←` `→` volume ±5%; `Space` preview; `m` mute; `Esc` close |
| Presets panel | `↑` `↓` move; `Tab` actions; `←` `→` choose an action; `Enter` start or run; `a` add; `e` edit; `d` delete; `r` restore defaults; `Esc` close |
| Form | `Tab` next field; `Enter` save; `Esc` cancel |
| Update dialog | `Tab` switches setup/portable; `←` `→` actions; `↑` `↓` `PgUp` `PgDn` `Home` `End` or the wheel scroll the release notes; `Enter` run; `g` GitHub; `Esc` later |
| Any screen | `Shift+Q` closes the TUI and the daemon (tray and mini timer included); `q` closes only the TUI. While typing a preset name, `Shift+Q` types `Q` |

In the app, key hints use the `[key] action` format, for example `[Space] pause`.

Duration formats: `25` = 25 minutes · `12:30` = 12 min 30 s · `90s` · `1h30m` · `1:02:03`.
The accepted range is one second to 24 hours.

> In idle, `s` and `m` add a unit when the input already contains a number (`90s`, `25m`);
> on an empty (or otherwise incomplete) input, `s` opens Statistics and `m` toggles the sound.
> `v` opens the sound panel from any screen. `M` toggles the Windows mini timer in the main
> screens (idle, running, paused and finished). `q` leaves the daemon running; use `Shift+Q`
> to close the TUI and the daemon together.

## Tray

On Windows, left click opens the TUI, or brings the open one to the front. Right click opens the menu.

When Windows Terminal is your default terminal, **Open Tuick-Tack** starts the TUI in a new Windows Terminal window with your default profile, so the `none` background keeps your acrylic, opacity or background image. The app only reads that profile; it never writes `settings.json` or changes your terminal colors. If Windows Terminal is not the default, it uses a normal console window. Opening `tuick-tack.exe` or the Start Menu shortcut directly does the same: the first console window is replaced by one with your default profile (a brief window may appear first).

```text
Tuick-Tack · 04:32 remaining
Check for updates
─────────────────────────────
Open Tuick-Tack
Start            ▸ 5 min · 10 min · 15 min · Custom…
Pause / Resume
Stop
Sound            ✓
Sound settings   ▸ Chime · Marimba · Digital · Deep · Preview
Volume 80%       ▸ -5% · +5% · 0% · 25% · 50% · 75% · 100%
Start with Windows
Mini timer       ✓
─────────────────────────────
Quit
```

The tray owns the timer. Closing the TUI leaves the countdown and the selected sound alive,
and **Quit** stops everything and closes any open TUI. A daemon restart loses the active
countdown; persistence is future work.

## Mini timer

Windows can draw a small countdown over the free left area of the taskbar. It shows only the
remaining time in the same pixel font as the TUI and a progress bar, using the active theme.
The **Mini timer** checkbox in the tray turns it on or off and is saved in the config; it is
on by default. Press `M` in the TUI to toggle it without opening the tray, or use **Hide mini
timer** in the overlay's context menu.

The widget appears while a countdown is running, paused or finished. When it opens or
closes it fades over about 160 ms while sliding a few pixels vertically, and it respects
Windows' animation setting (with animation effects off it appears and disappears instantly).
When the countdown finishes, the `00:00` screen stays for ten seconds and then the widget
hides itself, matching the TUI celebration's auto-close; starting another timer brings it
back. As it reaches zero, the widget glows and a one-shot, two-wave confetti burst falls
from the bar over the digits. It hides in idle, when the TUI window has focus, when the
taskbar is auto-hidden or in fullscreen, and when a Start menu or shell flyout is in front.
Left click opens or focuses the TUI; right click offers Open, Pause/Resume, Stop and
**Hide mini timer**. Hovering over the widget turns the pointer into a hand and fades the
border and panel into the theme accent over about 160 ms, without activating the window. It
is an independent overlay, not an Explorer integration: it does not support dragging,
vertical or secondary taskbars, and it may cover a Widgets button that shares that left
slot. macOS has no overlay; its menu bar already shows the remaining time.

## Configuration

The paths below are for Windows. The frozen macOS port keeps its own data under `~/Library`;
see `SPEC.md` §8.

- **Presets, theme and sound settings:** `%APPDATA%\tuick-tack\config.json`.
- **Session history for the Statistics tab:** `%APPDATA%\tuick-tack\history.jsonl`.
- **Logs, sound cache and update downloads:** `%LOCALAPPDATA%\tuick-tack\` (`sounds\`, `logs\`, `updates\`).
- **Installed copy (Windows):** `Documents\tuick-tack` when you use the installer; the uninstaller lives
  in the same folder and in Windows Settings. Uninstalling keeps your config and logs.
- **Portable copy:** wherever you saved `tuick-tack.exe`.
- **Autostart:** only if you enable it — `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
  value `TuickTack`, pointing to `tuick-tack.exe --daemon --hidden`. No admin rights.
- **Notification identity (Windows):** `HKCU\Software\Classes\AppUserModelId\Juankyyy.TuickTack`
  (`DisplayName=Tuick-Tack`), registered the first time a Windows toast is shown. It is not app
  state; no admin rights.

```json
{
  "version": 7,
  "theme": "midnight",
  "background": "aurora",
  "readable_text": true,
  "mini_timer": true,
  "sound": true,
  "sound_name": "digital",
  "volume": 80,
  "window_width": 140,
  "window_height": 45,
  "window_position": { "x": 93, "y": 100 },
  "presets": [
    { "id": "p5",  "label": "5 min",  "seconds": 300 },
    { "id": "p10", "label": "10 min", "seconds": 600 },
    { "id": "p15", "label": "15 min", "seconds": 900 }
  ]
}
```

The daemon is the only writer and writes atomically. A corrupt config is backed up as
`config.bad-<timestamp>.json` and replaced with defaults. Older configs are migrated to version 7:
version 1 gets Digital at 80%; versions 2 and 3 keep their sound, volume and size; versions 1–4
start with the original Aurora background; versions 1–5 enable `readable_text`; versions 1–6
enable the mini timer. The window position stays unset until you move the TUI.

`readable_text` defaults to `true` and controls the black reading bands used by `none`; `n`
toggles it.

The remembered size is columns by rows and the position is the window's top-left in screen
pixels. Both are restored when the TUI starts, including in Windows Terminal; terminals that
do not support the resize sequence keep their own size. A TUI running inside a terminal tab
never moves the whole window. While the window is maximized or minimized its size and
position are not recorded, so the last normal placement is kept; the maximized state itself
is not restored. This feature is Windows-only.

`background` is `aurora` (the original), `stars`, `fireflies`, `rain`, `waves`, `grid` (shown
as **Sunset**) or `none`. Press `b` in the TUI to cycle; the daemon stores the choice and every
open TUI picks it up. The theme only changes the palette, so any background works with Midnight,
Ember and Mono. Aurora keeps the original dust field and Tide shares it; Stars, Fireflies, Rain,
Sunset and none are clean or carry their own particles.

`none` leaves the terminal background untouched: the app paints no empty cells and never sets
the terminal background or foreground color, so a custom background, image or acrylic shows
through. UI panels, buttons, the progress bar, modals and the countdown block shadow keep their own
backgrounds; the shadow turns light gray when the terminal background is light, so the dark digits
never sit on a near-black shadow. The finish flash is skipped while the confetti still plays.

With `none`, the text that floats directly on the terminal (wordmark, version, state, subline,
label, meta, input, key hints, update notice, status and compact fallback) gets a tight black
backing with one cell of margin and light theme ink. Press `n` to turn the bands off; the daemon
remembers the choice. The countdown, progress bar, chips, buttons and modals never get a band.
With the bands off, the TUI asks the terminal for its real background color (OSC 11) and adapts
the stage text and countdown to a light background; a dark terminal, no reply or any other
ambient background keeps the active palette.

The daemon appends one line per finished countdown to `history.jsonl` (local time, planned and
real elapsed time, label and outcome: completed, stopped or replaced). It keeps the newest 2000
records and trims the file atomically; an unreadable file is backed up as
`history.bad-<timestamp>.jsonl` and the history starts empty. The Statistics tab reads a
precomputed summary over the pipe, so the TUI never parses the history. Nothing leaves the
machine and there is no telemetry.

### CLI flags

```text
tuick-tack --version
tuick-tack --check-updates
tuick-tack --install-startup
tuick-tack --uninstall-startup
tuick-tack --daemon            # background daemon with a console (development)
tuick-tack --daemon --hidden   # daemon without a console (used by autostart)
```

## Updates

The daemon checks the latest GitHub release once at startup and every 24 hours. When a newer
version exists, the TUI shows a moving pastel-rainbow `[Update available →]` after the wordmark
and its version. Clicking it, or pressing `u` once the release is known, opens the update
dialog: it shows `vCurrent → vLatest` and the action for this copy. You can install the setup
or download the portable `.exe`. The tray gets `Update available: vX.Y.Z`, which opens the
release page.

On Windows, the dialog shows the copy you are using first, selected and marked `Installed`
(setup) or `In use` (portable). Use `Tab` to switch to the other type; if you choose it, the
dialog warns that it is not the copy you use and that your settings carry over. The horizontal
action row stays below: `install` or `download`, `github` and `later`.

When the release publishes notes, the dialog also opens a `What's new in vX` panel above the
type row: the first eight lines at 80×24 and sixteen at 120×40, with a scrollbar. Read the rest
with `↑`/`↓`, `PgUp`/`PgDn`, `Home`/`End` or the mouse wheel; on Windows `Tab` keeps switching
the copy without losing the scroll position. Releases without notes keep the shorter dialog.

- **Install update:** downloads `tuick-tack-setup.exe`, verifies it and runs the same
  per-user installer silently. The app closes, updates the copy in `Documents\tuick-tack`
  and reopens. If a countdown is running or paused, the TUI warns that the countdown will be
  lost before it continues.
- **Download portable:** downloads the new `tuick-tack.exe` into Downloads as
  `tuick-tack-vX.Y.Z.exe`, verifies it and leaves the current app running. Use it when you
  prefer the portable copy.
- **macOS (frozen port):** the dialog has one `download` action for `tuick-tack-macos-universal.zip`; the
  daemon verifies it, saves it in Downloads as `tuick-tack-vX.Y.Z-macos-universal.zip` and
  reveals it in Finder. Nothing is replaced automatically: quit the menu bar app, unzip the
  file and replace `Tuick-Tack.app`.

Feedback and safety:

- Nothing downloads until you confirm. The TUI shows download, verification and installation
  progress; a failed check or download keeps the old version and shows a short error.
- The daemon verifies the SHA-256 from the release asset digest or the platform checksum file
  (`SHA256SUMS.txt` on Windows; the frozen macOS port uses its own zip checksum) before it
  runs or saves anything. A mismatch is discarded.
- The installed copy closes the daemon and the TUI gracefully, runs the installer with
  `/VERYSILENT` and reopens the TUI. The portable copy writes the new `.exe` to Downloads; it
  does not restart anything. On the frozen macOS port the zip is saved to Downloads and
  revealed in Finder; the app never replaces itself.
- The installer preserves an existing `Start with Windows` setting. User data in
  `%APPDATA%\tuick-tack` is never touched by an install, update or uninstall. On the frozen
  macOS port the LaunchAgent and `~/Library/Application Support/tuick-tack` survive an app
  replacement.
- The first release with this flow must be installed manually once by people on an older
  version. From then on, the TUI can update itself.
- One anonymous HTTPS request per check to `api.github.com`, with
  `User-Agent: tuick-tack/<version>` and no other identifiers. A failed check (offline, proxy,
  rate limit) is silent and retried after six hours. Asset downloads happen only after you
  confirm.
- The automatic check runs in release builds only (`dev` builds skip it); the manual check
  works in any build. `tuick-tack --check-updates` runs one check and prints the result.
- The installer is per-user and unsigned; SmartScreen can warn when you download it. There is
  no code signing yet.

## Architecture

```text
        one code base, two processes (Windows: tuick-tack.exe)
┌────────────────────────────┐  pipe/socket  ┌────────────────────────────────────┐
│ TUI (no arguments)         │◄────JSON─────►│ daemon (--daemon)                  │
│ Bubble Tea v2              │    10 Hz      │ systray on the main thread         │
│ canvas + bigfont + fx      │               │ timer engine · IPC · config        │
│ keyboard, mouse, modals    │               │ sound · PCM gain · autostart       │
└────────────────────────────┘               └────────────────────────────────────┘
```

- The daemon is the single source of truth. The TUI renders snapshots and interpolates
  locally for smoothness; it never keeps timer truth of its own.
- Windows uses one named pipe per user and session, one daemon per session (named mutex);
  the frozen macOS port uses a `0600` Unix socket in the cache directory and `flock` locks. The
  JSON protocol is the same.
- Shared code never imports a platform API. Each platform capability has a `_windows.go` and
  a `_darwin.go` leaf (IPC, instance lock, sound, autostart, launch, notifications, install,
  update); the macOS leaves are frozen. See the reference table in `SPEC.md` §4.
- The pipe protocol is versioned. The TUI reconnects every second and disables commands
  while offline. The update messages are additive, so an older daemon or TUI still works.
- The daemon checks the latest GitHub release once a day, broadcasts it to the TUI and the
  tray, and downloads an update only when the TUI asks. The downloader verifies
  SHA-256; Windows can run the setup helper or save the portable `.exe`, and the frozen macOS
  port saves the universal zip to Downloads and reveals it in Finder.
- On finish the daemon increments `finish_seq` once, plays the selected sound at the
  configured volume and updates the tray; the TUI celebrates when it sees the new
  sequence and dismisses the finished screen on its own after ten seconds (a manual close or
  repeat wins; an open modal postpones it until the modal closes).
- The daemon also records finished sessions and aggregates them for `get_stats`; the
  Statistics tab is a view over that summary and adds no timer truth of its own.

```text
cmd/tuick-tack        single entrypoint: TUI, daemon (--daemon) and update helper
internal/             engine, IPC, config, daemon, tray, minitimer, TUI, stats, update, install metadata
macos/                frozen Info.plist, launcher and README for Tuick-Tack.app
installer/            Inno Setup script and branded wizard images (Windows)
assets/               sounds, icons, banner and README screenshot
tools/                sound, icon, banner and installer-image generators
winres/               executable icon and version resources (.syso, Windows)
scripts/build.ps1     portable and installer release build (Windows)
scripts/build-macos.sh frozen universal .app build and zip (manual only)
```

## Development

On Windows, development requires Windows 10/11, Go 1.26+ and PowerShell 7. CGO is disabled,
so no C compiler is needed. Building the installer also needs Inno Setup 6
(`winget install -e --id JRSoftware.InnoSetup`). The frozen macOS build needs Xcode Command Line
Tools: cgo links the menu bar through systray; `CGO_ENABLED=0` still builds the headless
fallback. Never run `go generate ./...` on macOS or in CI: it includes the Windows `.syso` and
the Inno images.

```powershell
go mod download                  # install dependencies
go run ./cmd/tuick-tack          # TUI (starts the hidden daemon)
go run ./cmd/tuick-tack --daemon # daemon in the foreground
go test ./...                    # tests
go vet ./...                     # lint
go generate ./...                # Windows only: sounds, icons, banner, .syso
```

From Windows, an optional check for the frozen macOS port (not part of CI) is:

```powershell
$env:CGO_ENABLED="0"; $env:GOOS="darwin"; go build ./...; go vet ./...; Remove-Item Env:GOOS
```

### Build from source

```powershell
.\scripts\build.ps1                              # dev portable build into dist/
.\scripts\build.ps1 -Version v0.3.0              # release portable build
.\scripts\build.ps1 -Version v0.3.0 -Installer   # portable + setup + checksums
```

The script regenerates the assets, checks formatting, runs vet and the tests, and writes:

```text
dist/
  tuick-tack.exe
  tuick-tack-setup.exe   # with -Installer
  SHA256SUMS.txt
```

Run `dist\tuick-tack.exe` for the portable copy, or run `dist\tuick-tack-setup.exe` to
install it for the current user. Inno Setup is only needed to build the setup, not to run
the app.

### macOS build (frozen, manual)

The macOS workflow and script are kept for an occasional manual build; they do not run on push
and are not part of a Windows release. Dispatch `.github/workflows/macos.yml` by hand when a
macOS build is explicitly requested.

```bash
./scripts/build-macos.sh build -version v0.0.0 -arch arm64
./scripts/build-macos.sh package -version v0.0.0 \
  -arm64 dist/tuick-tack-darwin-arm64 -amd64 dist/tuick-tack-darwin-amd64
```

`build` compiles one architecture with cgo; `package` combines the two with `lipo`, builds
`dist/Tuick-Tack.app` from `macos/` and `assets/icon.iconset`, signs it ad-hoc and writes
`dist/tuick-tack-macos-universal.zip`, its `.sha256` and `dist/SHA256SUMS.txt`. It needs
`lipo`, `iconutil`, `codesign` and `ditto`, all part of macOS. Dispatching the workflow with a
`tag` attaches the zip and its `.sha256` to that release.

- Behavior and full design: [`SPEC.md`](SPEC.md)
- Conventions and workflow: [`AGENTS.md`](AGENTS.md)

## Troubleshooting

| Symptom | What to check |
|---|---|
| TUI cannot connect | The TUI starts the daemon itself; check `%LOCALAPPDATA%\tuick-tack\logs\tuick-tack.log`. |
| No sound | Check the system volume and output device, that **Sound** is checked in the tray, and that the selected sound is not at 0%. The countdown still finishes visually. |
| Tray icon missing | Windows hides new tray icons; open the overflow chevron. |
| Very small window | Below 48×22 the TUI switches to a plain-text compact view (clock, label, state and essential keys); the styled layout returns above that size. |
| Presets disappeared | A corrupt config is backed up and defaults are restored. Look for `config.bad-*.json` in `%APPDATA%\tuick-tack`. |
| No update notice | Release builds check once at startup and every 24 h; `dev` builds do not. Run `tuick-tack --check-updates` and check the daemon log. |
| No Windows notification | Check **Settings > System > Notifications** for Tuick-Tack and turn off Focus Assist. The tray entry still shows the result. |
| Update download fails | Check the daemon log in `%LOCALAPPDATA%\tuick-tack\logs\tuick-tack.log`. The old version stays installed; open the release page and retry. |
| Update setup fails | The helper logs to `%LOCALAPPDATA%\tuick-tack\logs\update.log` (the Inno log is `setup.log` next to it). The previous copy is reopened when possible. |
| Update lost a countdown | The TUI warns before restarting when a timer is running or paused; the timer lives in memory and is not restored. |
| SmartScreen warning | Expected for an unsigned executable and installer. |
| Mini timer missing | Check **Mini timer** in the tray, that a countdown is running/paused/finished, that the TUI does not have focus and that the taskbar is not auto-hidden or fullscreen. Vertical and secondary taskbars are not supported. |
| Mini timer covers the Widgets button | It occupies a fixed slot on the left. Turn it off in the tray or with **Hide mini timer**; there is no drag or offset setting yet. |
| Mini timer stays hidden with auto-hide on | Expected unless the cursor is over the taskbar; the widget follows the bar so it never keeps it open. |
| Timer lost after a crash | The timer lives in memory; restart the daemon and start again. Persistence is out of scope for this release. |
| **macOS (frozen port)** | The rows below describe the frozen port, not the Windows release. |
| Gatekeeper blocks the macOS app | The build is ad-hoc signed and not notarized. Right-click `Tuick-Tack.app` > **Open**, or run `xattr -dr com.apple.quarantine Tuick-Tack.app`. |
| macOS menu bar icon missing | Check that the app is still running in Activity Monitor; macOS may hide menu bar items when space runs out. There is no Dock icon by design (`LSUIElement`). |
| No macOS notification | Allow notifications for Tuick-Tack in **System Settings > Notifications**; the menu bar still shows the update state. |
| macOS autostart stopped after moving the app | The LaunchAgent is rewritten on the next launch; keep the app in a stable location such as `~/Applications`. |
| macOS update downloaded but nothing changed | Expected: the zip is only downloaded and revealed in Finder. Quit the menu bar app, unzip it and replace `Tuick-Tack.app`; config and history stay in `~/Library`. |
| Window placement not restored on macOS | Expected: the macOS TUI never moves or resizes the terminal. |

## License

GPL-3.0. See [`LICENSE`](LICENSE).
