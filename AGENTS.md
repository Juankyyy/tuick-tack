# Agent instructions

## Project

**Tuick-Tack** is a Windows countdown app for personal focus sessions: a modern terminal UI (TUI) plus a background tray daemon that owns the timer, presets, sound and system-tray menu.

- Stack: Go 1.26+ (pinned Bubble Tea v2/go-winio require it), Bubble Tea v2.1.1, Lipgloss v2.1.0, `fyne.io/systray` v1.12.2, `github.com/Microsoft/go-winio` v0.6.4, `golang.org/x/sys/windows`; `CGO_ENABLED=0`.
- App code lives in `cmd/` and `internal/`. Windows-specific code lives in `internal/tray/`, `internal/ipc/` and the `*_windows.go` files under `internal/daemon/`.
- Platform features that cannot live in the shared layer stay in the native layer behind small, documented bridges: systray/menu (`internal/tray`), `PlaySoundW` (`internal/daemon/sound_windows.go`), registry/autostart (`internal/daemon/autostart_windows.go`) and named pipes (`internal/ipc`).
- Current state: v1 implemented in `dev` (M0–M5 complete) plus the selectable sound pack and volume; `SPEC.md` is the functional source. Next work is polish/QA; releases are published on GitHub when the user asks for one.
- Generated assets (`assets/sounds/*.wav`, `assets/icon.ico`, `assets/icon.png`, `assets/icons/*.ico`, `assets/banner.txt`, `assets/banner-compact.txt`, `assets/banner.png`, the `.syso` file under `cmd/`) are committed; do not hand-edit them. Regenerate with `go generate ./...` / `scripts/build.ps1`. The branding banner comes from the same 5×7 bigfont grid as the countdown and is not embedded in the executable: `banner.txt` is the 2x ASCII (118 columns) and `banner-compact.txt` the 1x version (59 columns).

## Commands

Run from the repository root (PowerShell on Windows):

```bash
go mod download                # install dependencies
go vet ./...                   # lint
go build ./...                 # compile / typecheck
go test ./...                  # tests
go run ./cmd/tuick-tack          # TUI (console; starts the hidden daemon)
go run ./cmd/tuick-tack --daemon # daemon (console in dev; --hidden at sign-in)
go fmt ./...                     # format
go generate ./...                # regenerate sounds/icons and .syso resources
.\scripts\build.ps1              # vet + test + build the single exe into dist/
.\scripts\build.ps1 -Version v1.0.0
```

- Hot reload: none. Restart the TUI; the daemon can stay running and keeps the timer.
- CI: not configured yet. When added: GitHub Actions on `windows-latest` running `go vet ./...`, `go test ./...` and `go build ./...`; a red check blocks merges.

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
  daemon/               wiring, sound, autostart, logging
  tray/                 systray menu, icons, tooltip
  tui/                  Bubble Tea model/view, canvas, bigfont, theme, fx
assets/                 sounds/*.wav, icon.ico, icons/, banner.txt, banner-compact.txt, banner.png, screenshot.png
tools/                  gen-sounds, gen-icons, gen-banner
scripts/build.ps1
winres/                 go-winres metadata
```

- State management: the **daemon is the single source of truth** (timer, presets, theme, sound selection, volume, tray). The TUI is a client; it renders snapshots and interpolates locally for smoothness. `finish_seq` dedupes celebrations. No client-side timer truth, no external state library.
- Local persistence: JSON config at `%APPDATA%\tuick-tack\config.json`; the daemon is the only writer; atomic writes; no DB. The Windows registry is used **only** for autostart (`HKCU\...\Run`), never for app state. Do not add SQLite or a second config store.
- Localization: all user-facing copy lives in `internal/tui/copy`; UI code reads it via `copy.T(...)`. Do not hardcode labels, tooltips, dialogs or toasts in widgets. The v1 UI is English; if Spanish is added later, its messages live in the same package.
- Dependencies pinned for a reason keep a comment next to them; do not bump until the packages are compatible. Bubble Tea/Lipgloss v2 use the `charm.land/...` module paths and the v2 API; `systray` v1.12.2 is the Fyne fork with left/right-click callbacks and package-level menu state.
- Keep time-dependent logic testable by injecting a clock; never call `time.Now()` in domain logic that has tests. `time.Now()` belongs in `main`/wiring; tests use a fake clock. The engine relies on the monotonic reading inside `time.Time`.
- Config schema version is `2`. Bump it and add a migration before adding/changing fields; back up a corrupt config and fall back to defaults.
- No cgo: build with `CGO_ENABLED=0`. `systray` and `go-winio` are pure Go on Windows.
- No wide runes or emoji in the TUI. Every glyph must be one terminal cell; test layouts at 80×24 and 120×40.
- All TUI rendering goes through `internal/tui/canvas`; do not scatter raw ANSI outside the canvas serializer.
- `systray.Run` must run on the main goroutine; do not call systray APIs before `onReady`. The rest of its API is safe from other goroutines.
- The daemon must remain fully usable with the TUI closed (tray, timer, selected sound at the configured volume).
- The IPC protocol is versioned. Bump the protocol version for incompatible message changes; do not silently break the TUI/daemon pair.
- `PlaySoundW` uses a WAV file path with `SND_ASYNC`; do not use `SND_MEMORY` (it does not combine with async). Volume is applied by scaling the PCM samples into a cached WAV; never change the system or wave-out device volume.
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

- **Artefacto**: carpeta `dist/` con un único `tuick-tack.exe` (TUI de consola y daemon de
  bandeja con `--daemon`), generada por `scripts/build.ps1 -Version vX.Y.Z`. Portable, sin
  instalador, sin auto-update y sin firma. SmartScreen puede avisar la primera vez.
- **Publicación**: siempre en GitHub, a petición del usuario y desde `main`. Tag `vX.Y.Z`
  (patch por defecto si no se indica versión); adjuntar el `.exe` y `SHA256SUMS.txt`. Notas
  en inglés, manuales, con el formato de las releases anteriores.
- **Procedimiento**: verificar `dev` (diff `main..dev`, `go vet`, `go test`, `go build`),
  actualizar docs, merge/push a `main`, tag, compilar desde `main`, `gh release create`.
- **Nada automático**: no hay workflow de release, secrets ni artefactos firmados. La
  publicación la hace el agente a mano con `gh`; no añadir CI de release sin pedirlo.
- **Nunca** push/tag/merge a `main` por iniciativa propia; la única excepción es una
  release pedida por el usuario.

## Antes de decir que algo está terminado

- [ ] `go vet ./...`, `go test ./...` y `go build ./...` (o `scripts/build.ps1`) en verde.
- [ ] Assets generados regenerados y commiteados; `.syso` regenerado si cambió el icono.
- [ ] `SPEC.md` / `README.md` actualizados si cambió el comportamiento o la UI.
- [ ] La respuesta dice qué debe probar el usuario y qué quedó fuera.
- [ ] El `.exe` compila; el daemon no abre consola con `--hidden`; QA manual a 80×24 y 120×40.
- [ ] Sin `time.Now()` en lógica de dominio; sin runas anchas en la TUI.
