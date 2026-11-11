<div align="center">

# Tuick-Tack

**Tu countdown, en la terminal y en la bandeja.**

Presets, minutos libres, aviso sonoro y confeti. Windows 10/11.

<sub>En desarrollo · Go + Bubble Tea · GPL-3.0</sub>

</div>

---

## ✨ Lo que hace

| Función | Detalle |
|---|---|
| ⏱ **Presets** | 5, 10 y 15 min de serie. Crea los tuyos con nombre y duración. |
| ⌨️ **Minutos libres** | `25`, `12:30`, `90s`, `1h30m`. |
| ⏯ **Control total** | Pausa, reanuda, ajusta ±1 min y repite. |
| 📌 **Siempre a mano** | El countdown y el icono siguen aunque cierres la TUI. |
| 🖱️ **Bandeja** | Inicia un preset, pausa, detén o abre la TUI desde el icono. |
| 🔔 **Al llegar a 0** | Aviso sonoro de 4 s, icono en alerta y confeti a pantalla completa. |
| 🎨 **Tres temas** | Midnight, Ember y Mono. |
| 📦 **Portable** | Dos `.exe`, sin instalador, sin admin. |

## 🖥️ Un vistazo

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

> La UI de la v1 está en inglés; esta documentación está en español.

## 🚀 Empezar

> **En desarrollo.** Todavía no hay binarios: el primer hito es el spike de la bandeja.

**Cuando la v1 esté publicada** será tan simple como esto:

1. Descarga `tuick-tack.exe` y `tuick-tack-tray.exe` en la misma carpeta.
2. Abre `tuick-tack.exe`.
3. El daemon se inicia solo. Usa el icono de la bandeja; no hace falta instalar nada ni ejecutar como admin.

La primera vez, SmartScreen puede pedir confirmación porque el `.exe` no está firmado. Es esperado.

**Para desarrollo** (a partir de M0):

```powershell
go mod download
go run ./cmd/tuick-tack          # TUI
go run ./cmd/tuick-tack-tray     # daemon, sin ventana en release
.\scripts\build.ps1              # ambos .exe en dist/
```

Para usar la app: Windows 10/11. Para compilar: Go 1.24+. Windows Terminal recomendado.

## ⌨️ Controles

| Acción | Tecla |
|---|---|
| Escribir una duración | dígitos, `:`, `h`, `m`, `s` |
| Empezar | `Enter` |
| Elegir preset | `←` `→` + `Enter` |
| Pausar / reanudar | `Space` |
| Cancelar | `Esc` (confirmación) |
| ±1 minuto | `+` / `-` |
| Repetir al terminar | `r` |
| Presets | `p` |
| Tema | `t` |
| Sonido | `s` |
| Ayuda | `?` |
| Cerrar la TUI | `q` |

Formatos de duración: `25` = 25 min · `12:30` = 12 min 30 s · `90s` · `1h30m`.

## 🖱️ Bandeja

Clic izquierdo: abre la TUI. Clic derecho: menú.

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

## ⚙️ Configuración

- **Presets, tema y sonido:** `%APPDATA%\tuick-tack\config.json`
- **Logs y caché:** `%LOCALAPPDATA%\tuick-tack\`
- **Autostart:** solo si lo activas (`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`).

Todo es local. Sin cuentas, sin nube y sin servicios en segundo plano más allá del daemon.

## 🛠️ Desarrollo

```text
cmd/tuick-tack        TUI
cmd/tuick-tack-tray   daemon
internal/             motor, IPC, config, bandeja, TUI
assets/               aviso sonoro e iconos
```

- Comportamiento y arquitectura: [`SPEC.md`](SPEC.md)
- Convenciones y flujo: [`AGENTS.md`](AGENTS.md)
- Tests: `go test ./...` · Lint: `go vet ./...` · Build: `.\scripts\build.ps1`

## 📄 Licencia

GPL-3.0. Ver [`LICENSE`](LICENSE).
