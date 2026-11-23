# Agent instructions

## Project

**Tuick-Tack** is a personal focus countdown app: a modern terminal UI (TUI) plus a background daemon that owns the timer, presets, sound and the tray/menu-bar menu. Windows 10/11 is the reference platform; the macOS port lives in `dev` (phases 0–2 done).

- Stack: Go 1.26+ (pinned Bubble Tea v2/go-winio require it), Bubble Tea v2.1.1, Lipgloss v2.1.0, `fyne.io/systray` v1.12.2, `github.com/Microsoft/go-winio` v0.6.4, `github.com/mtunjic/winrt-toast` v1.2.0 (pure-Go WinRT toasts), `golang.org/x/sys`; the Windows build stays `CGO_ENABLED=0`. macOS builds without cgo for the headless fallback and with cgo for the real menu bar.
- App code lives in `cmd/` and `internal/`. Platform code is split by file suffix (`_windows.go`, `_darwin.go`) and build tags; shared code never imports a platform API. The Windows leaves cover the systray, `ShellExecuteW`, WinRT toasts, named pipes, the registry, `PlaySoundW`, CreateProcess and the Windows Terminal/console window; the macOS leaves cover syscalls, the Unix socket, flock, the LaunchAgent, afplay and `open`.
- Platform features that cannot live in the shared layer stay in the native layer behind small, documented bridges: systray/menu (`internal/tray`; ICO on Windows, 32 px PNG on macOS, no-op without cgo), sound (`internal/daemon/sound.go` plus `play_windows.go`/`play_darwin.go`), autostart (`autostart_windows.go` registry / `autostart_darwin.go` LaunchAgent), IPC (`internal/ipc/pipe_windows.go` / `pipe_darwin.go`), instance locks (`internal/instance`), browser, notifications (WinRT / osascript), explorer, process and window helpers, and the installer/update helper. Inno Setup 6 is a Windows-only build-time dependency; macOS needs cgo only for systray.
- Current state: v1 complete on Windows plus the macOS port phases 0–2 (CI, platform layer without the menu bar, real menu bar with PNG icons). Next: the `.app` bundle and packaging, then the macOS update flow and docs/QA; `SPEC.md` is still the Windows functional source until the docs phase. Releases are published on GitHub when the user asks for one.
- Generated assets (`assets/sounds/*.wav`, `assets/icon.ico`, `assets/icon.png`, `assets/icons/*.ico`, `assets/icons/*.png`, `assets/banner.txt`, `assets/banner-compact.txt`, `assets/banner.png`, `installer/wizard-large.png`, `installer/wizard-small.png`, the `.syso` file under `cmd/`) are committed; do not hand-edit them. Regenerate with `go generate ./...` / `scripts/build.ps1`. The branding banner comes from the same 5×7 bigfont grid as the countdown and is not embedded in the executable: `banner.txt` is the 2x ASCII (118 columns) and `banner-compact.txt` the 1x version (59 columns).
- The tray states are a 7×7 pixel grid and the executable icon a 16×16 grid, drawn without antialiasing by `tools/gen-icons`; keep the ICO sizes, the 32 px PNG variants and the 2× HICON caveat from `SPEC.md` §12.

## Commands

Run from the repository root (PowerShell on Windows):

```bash
go mod download                # install dependencies
go vet ./...                   # lint
go build ./...                 # compile / typecheck
go test ./...                  # tests
go run ./cmd/tuick-tack          # TUI (console; starts the hidden daemon)
go run ./cmd/tuick-tack --daemon # daemon (console in dev; --hidden at sign-in)
go run ./cmd/tuick-tack --check-updates # one GitHub release check and exit
go fmt ./...                     # format
go generate ./...                # regenerate sounds/icons/banners, installer art and .syso resources
.\scripts\build.ps1              # vet + test + build the portable exe into dist/
.\scripts\build.ps1 -Version v1.0.0
.\scripts\build.ps1 -Version v1.0.0 -Installer # portable + Inno setup + checksums
```

- Hot reload: none. Restart the TUI; the daemon can stay running and keeps the timer.
- CI: GitHub Actions, `.github/workflows/ci.yml`, on every push and pull request.
  `windows-latest` is the reference and runs `go vet ./...`, `go test ./...` and
  `go build ./...` with `CGO_ENABLED=0`. `macos-15` (arm64) and `macos-15-intel`
  (amd64) run the same vet, tests and build without cgo, so any shared code that
  imports a Windows-only API fails there. A red check blocks merges.
  `.github/workflows/macos.yml` is the manual base of the macOS packaging workflow.
  Never run `go generate` or Inno Setup in CI: they are Windows-only.

## Architecture

```text
cmd/
  tuick-tack/           single entrypoint: TUI (default) and daemon (--daemon)
internal/
  buildinfo/            version metadata
  config/               config + presets load/save/validate
  duration/             duration parser
  engine/               timer state machine
  protocol/             IPC message types
  ipc/                  named-pipe server/client
  stats/                session history (JSONL) and the Statistics aggregates
  daemon/               wiring, sound, autostart, logging
  update/               GitHub release check, assets, SHA-256 downloader
  updater/              temporary helper that runs the setup and reopens the TUI
  install/              reads the Inno uninstall entry (installed vs portable)
  instance/             daemon/TUI mutex names and acquisition
  winproc/              CreateProcess helpers (hidden daemon, new console)
  wt/                   read-only Windows Terminal default profile + launch
  termwin/              locate and move the real console window (conhost/WT)
  explorer/             open the downloaded portable in Explorer
  browser/              ShellExecuteW for https links
  notify/               native Windows toasts (WinRT, AUMID)
  tray/                 systray menu, icons, tooltip
  tui/                  Bubble Tea model/view, canvas, bigfont, theme, fx, copy, notes
installer/              Inno Setup script and branded wizard images
assets/                 sounds/*.wav, icon.ico, icons/, banner.txt, banner-compact.txt, banner.png, screenshot.png
tools/                  gen-sounds, gen-icons, gen-banner, gen-installer
scripts/build.ps1
winres/                 go-winres metadata
```

- State management: the **daemon is the single source of truth** (timer, presets, theme, background, sound selection, volume, remembered TUI window placement, tray). The TUI is a client; it renders snapshots and interpolates locally for smoothness. `finish_seq` dedupes celebrations. No client-side timer truth, no external state library.
- Local persistence: JSON config at `%APPDATA%\tuick-tack\config.json`; the daemon is the only writer; atomic writes; no DB. Session history for the Statistics tab lives next to it in `%APPDATA%\tuick-tack\history.jsonl` (append-only JSONL, cap 2000, atomic trim): it is data, not settings, and does not make config.json a second store. The Windows registry is used **only** for autostart (`HKCU\...\Run`), the per-user AUMID registration (`HKCU\Software\Classes\AppUserModelId\Juankyyy.TuickTack`, `DisplayName`), the standard Inno uninstall entry (read-only from the app; written by the setup) and the read-only default-terminal delegation (`HKCU\Console\%%Startup`) used to choose the TUI window profile; never for app state. Do not add SQLite or a second config store.
- Localization: all user-facing copy lives in `internal/tui/copy`; UI code reads it via `copy.T(...)`. Do not hardcode labels, tooltips, dialogs or toasts in widgets. The v1 UI is English; if Spanish is added later, its messages live in the same package.
- Dependencies pinned for a reason keep a comment next to them; do not bump until the packages are compatible. Bubble Tea/Lipgloss v2 use the `charm.land/...` module paths and the v2 API; `systray` v1.12.2 is the Fyne fork with left/right-click callbacks and package-level menu state.
- Keep time-dependent logic testable by injecting a clock; never call `time.Now()` in domain logic that has tests. `time.Now()` belongs in `main`/wiring; tests use a fake clock. The engine relies on the monotonic reading inside `time.Time`.
- Config schema version is `6`. Bump it and add a migration before adding/changing fields; back up a corrupt config and fall back to defaults.
- No cgo: build with `CGO_ENABLED=0`. `systray` and `go-winio` are pure Go on Windows.
- No wide runes or emoji in the TUI. Every glyph must be one terminal cell; test layouts at 80×24 and 120×40.
- All TUI rendering goes through `internal/tui/canvas`; do not scatter raw ANSI outside the canvas serializer.
- `systray.Run` must run on the main goroutine; do not call systray APIs before `onReady`. The rest of its API is safe from other goroutines.
- The daemon must remain fully usable with the TUI closed (tray, timer, selected sound at the configured volume).
- TUI launch: when Windows Terminal is the default terminal, the tray and a direct launch reopen the TUI in a new Windows Terminal window with the user's default profile (`internal/wt`, read-only), so `none` keeps the user's acrylic, opacity or background image. Otherwise they fall back to a normal console window. Never write to Windows Terminal's settings or set the terminal's default colors (no OSC 10/11). Direct launches pass the internal `--launch-handled` marker so the relaunch cannot loop.
- The IPC protocol is versioned. Bump the protocol version for incompatible message changes; do not silently break the TUI/daemon pair.
- `PlaySoundW` uses a WAV file path with `SND_ASYNC`; do not use `SND_MEMORY` (it does not combine with async). Volume is applied by scaling the PCM samples into a cached WAV; never change the system or wave-out device volume.
- Updates are explicit and checksum-verified: `internal/update` checks the GitHub release once at startup and every 24 h and selects the stable assets `tuick-tack-setup.exe`, `tuick-tack.exe` and `SHA256SUMS.txt`. Nothing downloads until the TUI asks. The installed copy downloads the setup, verifies it and lets `internal/updater` run it silently after the daemon/TUI exit; the portable copy downloads the new `.exe` to Downloads. `internal/install` only reads the standard Inno uninstall entry to highlight the matching action. The tray's **Check for updates** and the TUI `u` key trigger the same manual check; only the tray path raises a native Windows toast (`internal/notify`). Do not move the download into the TUI: the daemon owns network, verification and the update shutdown.
- Truecolor is assumed; Windows Terminal is the reference terminal. `conhost` is a fallback, not a design target.

## Convenciones

- **Idioma**: código, comentarios, mensajes de commit y notas de release en **inglés**.
  La UI está en **inglés**; todo el copy vive en `internal/tui/copy`, nunca en los
  widgets. Documentación de trabajo (`SPEC.md`, `AGENTS.md`, `README.md`): puede estar en
  español, salvo que el usuario pida lo contrario.
- **Commits**: convencionales, en inglés, con scope (`feat(tui): …`, `fix(engine): …`,
  `docs(spec): …`, `chore`, `ci`, `refactor`, `test`). El cuerpo explica el *porqué*; el
  diff ya dice el *qué*. Sin commits `WIP` en `dev`.
- **Tests**: `_test.go` junto al paquete; table-driven; deterministas (sin `time.Sleep`);
  reloj y RNG inyectados; tests de IPC con pipes de nombre único y limpieza. Los tests de
  UI comprueban 80×24 y 120×40 y que no haya runas anchas.

## Flujo de trabajo (acordado con el usuario)

La fuente funcional es `SPEC.md`; este documento manda en el proceso.

1. **Ramas**: todo el trabajo nuevo en la rama `dev`. El repo arranca en `main`; crear
   `dev` al empezar la implementación. Hacer push a `dev` **no publica nada**. `main`
   permanece estable y **solo se toca con autorización explícita del usuario**, después de
   verificar el bloque o la funcionalidad en su máquina. El agente nunca hace merge ni push
   a `main` por iniciativa propia.
2. **Paso a paso**: espera a que el usuario pida módulos o cambios concretos; no generes
   la app entera de golpe. Al terminar, indica qué debe probar el usuario.
3. **Aviso previo**: no se hacen commits ni pushes sin avisar antes. Propón el mensaje y
   espera el OK. «Haz los commits» autoriza commit **y push a `dev`** a la vez, sin lanzar
   ninguna release: el trabajo nunca se queda solo en local.
4. **`main` y releases, solo a petición**: `main` permanece estable y el agente nunca hace
   merge ni push a `main` por iniciativa propia, ni siquiera con «Haz los commits».
   «Verifica todo lo nuevo para crear una nueva release» (o «prepara/saca una release») es
   una petición explícita: autoriza el merge/push de `dev` a `main`, el tag `vX.Y.Z` y la
   publicación en GitHub en ese mismo momento. No preguntar si la release es local o de
   GitHub: **siempre es una release de GitHub** y se crea directamente con
   `gh release create`. Si no se indica versión, se sube el patch.
5. **Versiones y notas**: no hay manifiesto de versión en el repo. `internal/buildinfo.Version`
   vale `dev` por defecto; las builds de release la fijan con `-ldflags "-X ..."`. Las notas
   van en inglés, se escriben a mano y se pasan a `gh release create` (`--notes-file`). Un
   pedido de release ya autoriza tag y release; no hace falta confirmarlo otra vez.
6. **Documentación como parte del cambio**: actualiza `SPEC.md` si cambia el comportamiento
   o la arquitectura, y `README.md` si el cambio es visible para el usuario.

## Releases

- **Artefacto**: carpeta `dist/` con `tuick-tack.exe` (portable) y `tuick-tack-setup.exe`
  (instalador por usuario en `Documentos\tuick-tack`) más `SHA256SUMS.txt`, generada por
  `scripts/build.ps1 -Version vX.Y.Z -Installer`. Sin firma; SmartScreen puede avisar.
  Mantener el portable como opción de la release; el setup es el recomendado.
- **Publicación**: siempre en GitHub, a petición del usuario y desde `main`. Tag `vX.Y.Z`
  (patch por defecto si no se indica versión); adjuntar los dos `.exe` y `SHA256SUMS.txt`.
  Notas en inglés, manuales, con el formato de las releases anteriores e indicando que
  quien venga de una versión sin actualizador debe instalar esta una vez.
- **Notas en la app**: el diálogo de actualización muestra la sección `## What's new` del
  `body` de la release, desde ese encabezado hasta el siguiente `##`; mantenerla al
  principio de cada release. Si falta, la app cae al body completo. `internal/update`
  recorta el body a 16 KiB y lo envía como `notes` (campo aditivo; no sube `Protocol`).
- **Procedimiento**: verificar `dev` (diff `main..dev`, `go vet`, `go test`, `go build`),
  actualizar `winres/winres.json` (FileVersion/ProductVersion) y regenerar el `.syso`,
  actualizar docs, merge/push a `main`, tag, compilar desde `main` con `-Installer`,
  `gh release create`. Comprobar que el setup compila con Inno Setup 6 (`ISCC.exe`).
- **Nada automático**: no hay workflow de release, secrets ni artefactos firmados. La
  publicación la hace el agente a mano con `gh`; no añadir CI de release sin pedirlo.
- **Nunca** push/tag/merge a `main` por iniciativa propia; la única excepción es una
  release pedida por el usuario.

## Antes de decir que algo está terminado

- [ ] `go vet ./...`, `go test ./...` y `go build ./...` (o `scripts/build.ps1 -Installer`) en verde.
- [ ] Assets generados regenerados y commiteados; `.syso` regenerado si cambió el icono; imágenes del instalador regeneradas con `go generate ./...`.
- [ ] `SPEC.md` / `README.md` actualizados si cambió el comportamiento o la UI.
- [ ] La respuesta dice qué debe probar el usuario y qué quedó fuera.
- [ ] El `.exe` compila; el daemon no abre consola con `--hidden`; QA manual a 80×24 y 120×40.
- [ ] Instalador probado (instalar, actualizar, desinstalar, autostart) y actualización TUI probada en copia instalada y portable, incluido el aviso de countdown y un fallo de red/checksum.
- [ ] Sin `time.Now()` en lógica de dominio; sin runas anchas en la TUI.
