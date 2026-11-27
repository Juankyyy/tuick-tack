# Tuick-Tack — SPEC

> **Estado:** v1 Windows + mini timer; port macOS congelado · **Fecha:** 2026-11-26
> **Plataformas:** Windows 10/11 x64 (soportada) · macOS 13+ universal (congelada, builds manuales) · **Licencia:** GPL-3.0 (repo)
> **Stack:** Go 1.26+ + Bubble Tea v2 + Lipgloss v2 + systray + go-winio (Windows) / Unix socket (macOS, congelado)
> **Repo:** `tuick-tack` (v1 Windows; port macOS congelado en `dev`; builds locales `portable`/`setup`; el zip macOS solo se genera a mano)

> **Port macOS congelado:** no forma parte del CI, las releases ni la QA normal. Las secciones
> macOS se conservan como referencia para un build manual puntual; no son criterios de
> aceptación de Windows y no se amplían sin una petición explícita.

---

## 1. Resumen

Tuick-Tack es una app de escritorio para Windows 10/11 (el port macOS está congelado) con dos caras:

1. **TUI** en la terminal (Bubble Tea) para elegir presets, escribir minutos, ver el countdown, pausar, ajustar y gestionar presets.
2. **Icono en la bandeja** (menu bar en el port macOS congelado) gestionado por un daemon en segundo plano que es dueño del temporizador, del sonido y del menú.

El countdown, el sonido y el icono siguen funcionando aunque se cierre la TUI. Al llegar a 0 suena el sonido elegido al volumen configurado y, si la TUI está abierta, se celebra con confeti a pantalla completa. Presets por defecto 5/10/15 min, presets personalizados con nombre y duración, y entrada libre de minutos.

En Windows, el daemon también puede dibujar un **mini timer** superpuesto al hueco izquierdo de la barra de tareas: solo el tiempo restante y una barra de progreso, con el tema activo. Se oculta en idle, cuando la TUI tiene el foco, con la barra auto-oculta o en fullscreen.

Windows es la única plataforma soportada y no cambia de comportamiento. El port macOS está congelado: comparte la base de código y conserva sus bridges (bandeja, IPC, sonido, autostart y actualización), documentados en §4, pero no se compila en CI, no se publica con las releases normales y no se amplía.

## 2. Decisiones clave

| Área | Decisión |
|---|---|
| Lenguaje | Go 1.26+ (las versiones pinneadas de Bubble Tea v2 y go-winio lo requieren); `CGO_ENABLED=0` en Windows |
| Plataformas | Windows 10/11 x64 (soportada); macOS 13+ universal (congelada, build manual) sobre una sola base de código |
| TUI | `charm.land/bubbletea/v2` v2.1.1 + `charm.land/lipgloss/v2` v2.1.0 |
| Bandeja | `fyne.io/systray` v1.12.2: Go puro en Windows; cgo/Cocoa en macOS. Sin cgo en macOS, tray no-op |
| Mini timer | Overlay Win32 `WS_EX_NOACTIVATE` sobre el hueco izquierdo de la barra de tareas; toggle en la bandeja, menú del widget o `M`; solo Windows |
| IPC | Named pipe de `github.com/Microsoft/go-winio` en Windows; Unix domain socket en macOS; mensajes JSON iguales |
| Instancia | Mutex con nombre de Win32 en Windows; `flock` en macOS |
| Autostart | Registro `HKCU\...\Run` en Windows; LaunchAgent en `~/Library/LaunchAgents` en macOS |
| Notificaciones | WinRT toast con AUMID en Windows; `osascript` en macOS |
| Instalación | Inno Setup por usuario en Windows; `.app` portable en `~/Applications` o `/Applications` en macOS |
| Actualización | Setup o `.exe` portable en Windows; zip universal + `.sha256` y sustitución manual del `.app` en macOS |
| Win32 | `golang.org/x/sys/windows` (`PlaySoundW`, mutex, registro) |
| macOS nativo | syscalls de `golang.org/x/sys/unix`, socket Unix, `afplay`, `open` y LaunchAgent (congelado) |
| Icono del .exe | `go-winres` generando un `.syso` (solo Windows) |
| Arquitectura | Un ejecutable con dos modos (TUI y daemon), una sola cuenta atrás activa |
| Idioma | UI en inglés; código, comentarios y commits en inglés; documentación en español |
| Temas | `midnight` (por defecto), `ember`, `mono` |
| Aviso al terminar | Sonido elegible (1–5 s) con volumen 0–100 + icono de bandeja/menu bar en alerta + confeti en la TUI |
| Estadísticas | Historial local `history.jsonl` (2000 registros), agregados en el daemon y pestaña `s` de la TUI con overview, actividad, presets y sesiones recientes |
| Toast nativo | Solo para el resultado de una comprobación manual de actualizaciones: WinRT en Windows y `osascript` en macOS (congelado, sin clic); el aviso de fin sigue siendo sonido + TUI |

Las filas macOS de esta tabla describen el port congelado; no son requisitos de la release
Windows y solo se validan en un build manual pedido expresamente.

**Por qué un ejecutable y dos procesos:** un solo archivo evita que el usuario tenga que emparejar dos binarios. Por debajo siguen siendo dos procesos porque la TUI vive en una terminal: si fuese un solo proceso, cerrar la terminal mataría el icono, el countdown y el sonido. El daemon posee el estado; la TUI es una vista/controlador que se puede abrir y cerrar. En Windows ambos modos viven en `tuick-tack.exe`: sin argumentos arranca la TUI y con `--daemon` el daemon. En el port macOS congelado el mismo binario vive dentro de `Tuick-Tack.app/Contents/MacOS/` y el wrapper `launcher` arranca el daemon al abrir el bundle.

**Por qué Go + Bubble Tea:** `systray` cubre exactamente el requisito (menú, submenús, tooltip, clic izquierdo/derecho, actualización dinámica) sin bucle de eventos propio; Bubble Tea/Lipgloss permite iterar el acabado visual rápido. Rust/ratatui se descartó por más ceremonia en la bandeja y el render.

## 3. Objetivos y no objetivos

### Objetivos

- Countdown preciso (reloj monotónico), con pausa/reanudación exactas.
- Presets 5/10/15 min editables + presets personalizados persistidos.
- Entrada libre de duraciones (`25`, `12:30`, `90s`, `1h30m`).
- Icono de bandeja con menú contextual para lanzar cualquier preset.
- Chime de 3–5 s al llegar a 0.
- Celebración con confeti en la TUI al llegar a 0.
- Estilo cuidado: fondo ambiental a pantalla completa, cabecera de dos alturas, dígitos grandes con degradado y barra de progreso viva con checkpoints.
- El countdown y el sonido sobreviven al cierre de la TUI.
- La TUI, el daemon, los presets, los temas, los fondos, las estadísticas y el aviso de actualización en Windows; el port macOS congelado conserva la misma base y sus bridges.
- Mini timer opcional en Windows: tiempo restante y barra sobre el hueco izquierdo de la barra, oculto en idle y con la TUI enfocada.
- Configuración persistente, temas, autostart opcional, build reproducible del `.exe` portable y de un setup por usuario en Windows. El `.app` universal queda como referencia congelada y solo se genera a mano.

### No objetivos v1

- Varios timers simultáneos o cola de timers.
- Persistencia del timer en curso tras reiniciar el daemon.
- Linux (la app cubre Windows; macOS está congelado).
- Mini timer nativo o embebido en la barra de tareas; es un overlay independiente y no se inyecta en Explorer.
- Toast nativo de Windows para el fin del countdown (las comprobaciones de actualizaciones sí usan uno, ver §12).
- MSIX, descarga o instalación automática sin confirmación, firma de código.
- Notarización y Developer ID en macOS, auto-reemplazo silencioso de `Tuick-Tack.app` e instalador/desinstalador tipo Inno en macOS (fuera de alcance mientras el port siga congelado).
- Memoria de la posición y el tamaño de la ventana de la TUI en macOS (solo Windows).
- Hotkeys globales, integración con calendario, sync en la nube.
- UI mouse-first (el ratón funciona para chips/botones, pero el teclado manda).

## 4. Arquitectura

```
        una base de código, dos procesos
┌───────────────────────────┐  pipe /    ┌──────────────────────────────────────┐
│ TUI (sin argumentos)      │◄─socket───►│ tuick-tack --daemon                  │
│ TUI (consola)             │  JSON      │ systray (menu bar congelado)         │
│ Bubble Tea v2             │  frames    │ TimerEngine · IPC server · config    │
│ canvas + bigfont + fx     │            │ sonido · volumen · iconos · menú     │
└───────────────────────────┘            └──────────────────────────────────────┘
```

- **Ejecutable** = Windows `tuick-tack.exe`, app de consola. Sin argumentos arranca la TUI; con `--daemon`, el daemon; con `--daemon --hidden`, el daemon sin consola (autostart). En macOS el binario vive en `Tuick-Tack.app/Contents/MacOS/tuick-tack`; el wrapper `launcher` arranca el daemon al abrir el bundle y la TUI se abre desde el menu bar. El mismo binario incluye el modo interno `--apply-update`, que solo usa el helper de Windows.
- **Instalador** = `tuick-tack-setup.exe`, generado con Inno Setup. Instala por usuario en `Documentos\tuick-tack`, sin admin, con acceso al menú Inicio y desinstalador. El portable sigue siendo `tuick-tack.exe`. En macOS no hay instalador: el zip contiene `Tuick-Tack.app` y se mueve a `~/Applications` o `/Applications`.
- **Daemon** = proceso `--daemon`. Es la única fuente de verdad: motor, config, sonido, bandeja/menu bar. En Windows, al arrancar desde el registro libera su consola con `--hidden`; en macOS `--hidden` se acepta como no-op y el LaunchAgent arranca el binario con `--daemon --hidden`.
- **TUI** = proceso sin argumentos. Se conecta por el pipe o el socket; si el daemon no está, lanza una copia oculta de sí mismo con `--daemon` y reintenta. En macOS el daemon escribe `open-tui.command` en `~/Library/Application Support/tuick-tack/` y lo abre con `open`, para no pedir permisos TCC de AppleScript.
- Una sola TUI por sesión. En Windows el tray enfoca la ventana abierta; en macOS una segunda TUI activa Terminal con `open -a` como best effort (sin localizar la ventana) y sale. Los comandos afectan al único timer.
- **Mini timer (Windows):** el daemon crea una ventana `WS_POPUP` con `WS_EX_TOOLWINDOW|WS_EX_TOPMOST|WS_EX_NOACTIVATE` sobre el hueco izquierdo de la barra de tareas. Tiene su propio hilo y message loop (`runtime.LockOSThread`), separado de `systray`; consulta snapshot/config del daemon, se oculta en idle, con la TUI enfocada, en fullscreen, con auto-hide o con un flyout del shell delante, y pinta solo tiempo + barra con `canvas`/`bigfont`. No es una segunda fuente de verdad ni una integración nativa en Explorer.
- **IPC:** en Windows el pipe es local, por usuario y sesión: `\\.\pipe\tuick-tack-<usuario>-<sesión>`, con ACL para el usuario actual y acceso remoto denegado. En macOS es un Unix domain socket en `~/Library/Caches/tuick-tack/daemon-<uid>.sock`, con modo `0600`; si la ruta supera 100 bytes el daemon falla con un error claro (no hay fallback). Un socket huérfano se borra si el connect falla.
- **Instancia única:** mutex con nombre (`CreateMutexW`) + fallo al escuchar el pipe en Windows; `flock` sobre `daemon.lock`/`tui.lock` en `~/Library/Caches/tuick-tack` en macOS. Si ya hay daemon, el segundo proceso sale con código 0.
- Si el daemon se reinicia se pierde el timer en curso (documentado).

### Plataformas (macOS congelado)

La lógica compartida no llama a APIs de plataforma. Cada capacidad vive detrás de un bridge pequeño con una hoja `_windows.go` y otra `_darwin.go`; Windows es la única plataforma soportada. El port macOS está congelado: sus hojas `_darwin.go` se conservan para un build manual puntual, pero no se mantienen, no están en el CI normal y no son criterios de aceptación. No hay fork ni rama paralela: ambas comparten el mismo repositorio.

| Capacidad | Windows | macOS |
|---|---|---|
| TUI, daemon, motor, presets, temas, fondos, stats | Igual | Igual |
| IPC | Named pipe `go-winio`, ACL por usuario | Unix socket `0600` en `~/Library/Caches/tuick-tack/daemon-<uid>.sock` |
| Instancia única | `CreateMutexW` + pipe | `flock` de `daemon.lock`/`tui.lock` |
| Menú | Bandeja `systray`, ICO; clic izquierdo abre/enfoca la TUI | Menu bar `systray` con cgo y PNG 32 px; el clic abre el menú y `Open Tuick-Tack` abre la TUI |
| Mini timer | Overlay `WS_EX_NOACTIVATE` en el hueco izquierdo de la barra; toggle en la bandeja, menú del widget o `M` | No hay overlay; el menu bar ya muestra el tiempo |
| Segunda TUI | Enfoca la ventana existente | Activa Terminal con `open -a` (best effort) y sale |
| Sonido | `PlaySoundW` con WAV escalado | `afplay` con el mismo WAV escalado |
| Autostart | `HKCU\...\Run`, `--daemon --hidden` | LaunchAgent `com.juankyyy.tuick-tack.plist` |
| Notificación de update | WinRT toast con AUMID; el clic abre la release | `osascript display notification`; sin clic |
| Abrir TUI | Windows Terminal o consola; `termwin` coloca y enfoca | `.command` + `open`; activa Terminal sin localizar la ventana |
| Instalación | Inno Setup por usuario | `.app` en `~/Applications` o `/Applications` |
| Update | Setup o portable `.exe` | Zip universal + `.sha256`; descarga y Finder, sin reemplazo automático |
| Ventana de la TUI | Tamaño y posición recordados (`termwin`, `CSI 8`) | No soportado: el daemon omite `window` y la TUI no mueve el terminal |
| Perfil de terminal | Lee el `defaultProfile` de Windows Terminal | No aplica |
| Datos | `%APPDATA%` y `%LOCALAPPDATA%` | `~/Library/Application Support` y `~/Library/Caches` |
| Iconos | ICO multi-tamaño, rejilla 7×7 | PNG 32 px, rejilla 7×7 |
| Build | `CGO_ENABLED=0`, `.syso`, Inno | cgo para systray, `lipo`, codesign ad-hoc y zip (manual) |
| Firma | Sin firma; SmartScreen puede avisar | Ad-hoc, sin notarizar; Gatekeeper avisa en descargas |
| CI | `windows-latest` en push/PR | Solo manual (`workflow_dispatch`) |

Reglas de mantenimiento:

- El código compartido nunca importa `golang.org/x/sys/windows`, `go-winio`, `winrt-toast` ni `systray`; un build manual de macOS lo impide.
- Una capacidad nueva se declara en el bridge y se implementa en las hojas soportadas. Para Windows, la hoja `_windows.go` es la única obligatoria; el port macOS congelado no recibe features nuevas.
- No se duplica el daemon, la TUI ni la bandeja para macOS. El port congelado comparte la misma base.
- La tabla de paridad es referencia del port congelado y solo se actualiza si se toca ese port. Una feature de Windows está terminada con el CI de Windows verde; macOS solo se valida si se pide una release macOS.

## 5. Ciclo de vida

> Las menciones a macOS en este ciclo describen el port congelado; el flujo normal es Windows.

1. El usuario abre la app. En Windows es `tuick-tack.exe`; en macOS abre `Tuick-Tack.app`, cuyo wrapper arranca el daemon. Si ya hay una TUI (mutex `<pipe>-tui-mutex` en Windows, `flock` en macOS), en Windows la nueva instancia enfoca la ventana existente y sale; en macOS activa Terminal con `open -a` como best effort y sale (sin localizar la ventana).
2. La TUI intenta conectar al pipe o socket. Si falla:
   - Windows: lanza una copia oculta de `tuick-tack.exe --daemon` detached con `CREATE_NO_WINDOW`.
   - macOS: lanza el binario con `--daemon --hidden` con `Setsid` y stdio a `/dev/null`; el daemon escribe su propio log y se mantiene sin terminal.
   - Reintenta durante ~3 s (100 ms entre intentos).
3. El daemon arranca: logger, config, lock, engine + servidor IPC en goroutines, `systray.Run` en el hilo principal. En macOS el LaunchAgent opcional lo arranca al iniciar sesión.
4. La TUI recibe `hello` + config y renderiza. En macOS el `hello` omite `window` (no hay colocación de ventana).
5. Al llegar a 0, el daemon incrementa `finish_seq`, reproduce el sonido elegido y actualiza la bandeja o el menu bar. La TUI detecta el nuevo `finish_seq` y lanza la celebración.
6. `q` cierra solo la TUI; el daemon sigue. `Quit` en la bandeja/menu bar cierra todo: el daemon avisa `daemon_closing` a las TUIs conectadas y la TUI sale sin reintentar (si reintentase, `Ensure` relanzaría el daemon).
7. Al salir desde el menú: `systray.Quit()`, parar el loop, enviar `daemon_closing` a los clientes (se espera la escritura, con timeout), cerrar el pipe/socket y parar el sonido. Un cierre de systray sin `Quit` (logoff) no envía el aviso.

## 6. Motor del temporizador

Estados: `idle` → `running` ⇄ `paused` → `finished` → `idle` (dismiss/stop/start).

- Deadline monotónico: `deadline := time.Now().Add(duration)` (el `time.Time` de Go incluye lectura monotónica). `remaining = deadline.Sub(now)`.
- Pausa: `remaining` se guarda; `resume` fija `deadline = now + remaining`.
- Ticker de 100 ms. Al cruzar 0: estado `finished`, `finish_seq++`, se emite **una** vez.
- `adjust ±60 s`: mueve el deadline/remaining, con clamp a 0; si llega a 0, finaliza.
- `start` con un timer activo **reemplaza** el actual. La bandeja lo hace directo; la TUI muestra un toast “replaced”.
- `restart`: repite la última duración/etiqueta.
- `dismiss`: vuelve a idle; para el sonido.
- El estado `finished` se mantiene hasta dismiss/stop/nuevo start; la bandeja queda en alerta.
- Un solo timer. La UI no calcula la verdad: pinta el estado del daemon e interpola localmente entre snapshots para que la barra vaya suave.

Campos de estado:

```json
{
  "status": "running",
  "duration_ms": 300000,
  "remaining_ms": 271500,
  "label": "5 min",
  "preset_id": "p5",
  "finish_seq": 3,
  "end_at_unix_ms": 1762839120000
}
```

`end_at_unix_ms` es informativo para “ends at HH:MM”; la precisión se basa en el monotónico del daemon.

## 7. Protocolo IPC

- Transporte: named pipe de go-winio en Windows y Unix domain socket en macOS; los mensajes JSON consecutivos usan `json.Encoder`/`json.Decoder` sobre `net.Conn`. Los buffers del pipe de Windows son explícitos (64 KiB): con los tamaños por defecto una escritura puede esperar a que el cliente lea, y eso bloquearía los snapshots o el aviso final.
- Handshake: cliente `hello {protocol: 1}` ↔ servidor `hello {protocol: 1, app_version, config, update?, window?}`. `config` incluye el fondo actual; un daemon viejo lo omite y la TUI conserva su elección local. `window` es aditivo y lo envía el daemon de Windows (`{width,height}` en celdas y `position?` en píxeles; ceros si no hay tamaño guardado); el daemon de macOS lo omite porque no coloca la ventana. Su presencia indica que el daemon entiende `set_window_size`; un daemon viejo lo omite y la TUI no manda ese comando. La posición es opcional: nil significa que no hay ninguna guardada, que el daemon es viejo o que es macOS.
- El servidor acepta N clientes; broadcast de estado a 10 Hz mientras corre (y en cada cambio), y config solo al cambiar.
- Errores: `{ "type": "error", "code": "...", "message": "..." }`.
- La TUI reintenta la conexión cada 1 s si se cae; muestra banner de desconexión y deshabilita comandos.
- `daemon_closing` es la excepción: la TUI sale sin reintentar y sin lanzar un daemon nuevo, porque el cierre lo pidió el usuario desde la bandeja o el menu bar.
- Los mensajes de servidor desconocidos se decodifican como `IgnoredMessage` y se ignoran, para que un mensaje aditivo no rompa un cliente viejo. Por eso `update`, `update_progress` y `daemon_closing` no suben `Protocol`. El campo `notes` (cuerpo de la release, recortado a 16 KiB) es aditivo dentro de `update` y `update_result`: un daemon viejo no lo envía y una TUI vieja lo ignora. `install_update` y `download_portable` son comandos nuevos: un daemon viejo responde `invalid_command`, que la TUI muestra como actualización no disponible. `set_window_size` también es aditivo: la TUI solo lo envía si el `hello` trae `window`. `set_background` es aditivo: la TUI solo lo envía si el `config` del handshake trae un fondo no vacío; si no, cicla el efecto solo para esa sesión. `readable_text` es un puntero aditivo en `config` y `set_readable_text` un comando aditivo: la TUI solo lo envía si el puntero está presente; con un daemon viejo conserva el ajuste local.

Cliente → daemon:

| Comando | Campos |
|---|---|
| `start` | `seconds`, `label`, `preset_id?` |
| `pause` / `resume` / `stop` / `dismiss` / `restart` | — |
| `adjust` | `delta_seconds` |
| `preset_add` | `label`, `seconds` |
| `preset_update` | `id`, `label`, `seconds` |
| `preset_delete` | `id` |
| `presets_restore_defaults` | — |
| `set_theme` | `name` |
| `set_background` | `name` (`aurora`, `stars`, `fireflies`, `rain`, `waves`, `grid`, `none`) |
| `set_readable_text` | `enabled` (`bool`) |
| `toggle_mini_timer` | — (aditivo y sin campos; solo Windows) |
| `set_sound` | `enabled` |
| `set_sound_name` | `sound_name` |
| `set_volume` | `volume` |
| `set_window_size` | `width`, `height` (celdas; 1..1000), `position?` (`x`,`y` en píxeles; si falta, se conserva la guardada) |
| `preview_sound` | — |
| `stop_sound` | — |
| `check_update` | — |
| `install_update` | `force?` (obligatorio si hay countdown activo) |
| `download_portable` | — (en macOS descarga el zip universal y emite `update_progress` de tipo `portable`) |
| `get_state` | — |
| `get_stats` | — (pide el agregado del historial; solo responde al cliente que lo pide) |
| `quit_daemon` | — |

En macOS (port congelado) `install_update` responde `setup`/`failed`/`unsupported` y la TUI no lo ofrece; `download_portable` guarda `tuick-tack-vX.Y.Z-macos-universal.zip` en Downloads y el mensaje `done` lleva esa ruta. El resto de comandos es idéntico en las dos plataformas.

Daemon → cliente:

| Mensaje | Contenido |
|---|---|
| `hello` | `protocol`, `app_version`, `config`, `update?`, `window?` (`width`, `height`, `position?`; siempre presente en daemons de esta versión) |
| `state` | snapshot del §6 |
| `config` | `presets`, `theme`, `background`, `readable_text?`, `sound`, `sound_name`, `volume` |
| `update` | `latest`, `url`, `setup?`, `portable?`, `notes?` |
| `update_result` | `current`, `latest?`, `url?`, `setup?`, `portable?`, `notes?`, `error?` |
| `update_progress` | `kind` (`setup`/`portable`), `phase`, `received?`, `total?`, `path?`, `error?` |
| `stats` | `daily` (189 días locales, para cubrir las 26 semanas del heatmap con holgura), `hourly` (24), `weekday` (7, lunes primero), `labels` (top 8), `records`, `recent` (20, más reciente primero), `totals` (`d7`/`d30`/`d90`/`all`), `retained` |
| `daemon_closing` | `reason?` (`quit`/`update`; la TUI sale sin reintentar) |
| `error` | `code`, `message` |

`notes` es el `body` de la release (Markdown UTF-8) recortado a 16 KiB. No es estado de la app: la TUI extrae su sección `What's new`, la limpia y la pinta en el diálogo; el daemon solo lo cachea y lo transporta.

`mini_timer` **no viaja por el protocolo**: la TUI no conoce su estado (la bandeja es la fuente visual), pero puede alternarlo con el comando aditivo `toggle_mini_timer`, sin campos. Un daemon viejo responde `invalid_command`; no sube `Protocol`.

`get_stats` y `stats` son aditivos: un daemon viejo responde `invalid_command` y la TUI cierra la pestaña con un aviso; un cliente viejo ignora `stats`. El daemon responde solo al cliente que lo pidió, como `update_result`. La TUI pide el agregado al abrir la pestaña, al reconectar y cuando un snapshot muestra un stop o un reemplazo con la pestaña abierta (un final la cierra para celebrar); el resto del tiempo no hay tráfico extra.

El tamaño y la posición de ventana no se difunden: son geometría del cliente, no estado compartido. El daemon los guarda en config (solo Windows) y los devuelve en el siguiente `hello`; no hace broadcast ni refresca la bandeja.

Validación: duración 1..86400 s; etiqueta 1..32 caracteres; máximo 16 presets; tema válido; fondo válido (`aurora`, `stars`, `fireflies`, `rain`, `waves`, `grid`, `none`); sonido válido (`chime`, `marimba`, `digital`, `deep`); volumen 0..100; tamaño de ventana 1..1000 por 1..1000 celdas; posición entre -32768 y 32767 en ambos ejes. Comando inválido → error, sin cambiar estado.

## 8. Configuración y presets

Rutas (por usuario, sin admin):

| Datos | Windows | macOS |
|---|---|---|
| Config | `%APPDATA%\tuick-tack\config.json` | `~/Library/Application Support/tuick-tack/config.json` |
| Historial | `%APPDATA%\tuick-tack\history.jsonl` | `~/Library/Application Support/tuick-tack/history.jsonl` |
| Caché y logs | `%LOCALAPPDATA%\tuick-tack\` (`sounds\`, `logs\`, `updates\`) | `~/Library/Caches/tuick-tack/` (`sounds/`, `logs/`, `updates/`) |
| Autostart | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` | `~/Library/LaunchAgents/com.juankyyy.tuick-tack.plist` |

Rutas de config e historial vía `os.UserConfigDir`/`os.UserCacheDir`; el historial es JSONL solo-añadir, escrito únicamente por el daemon. `window_width`/`window_height` y `window_position` solo se usan en Windows; en el port macOS congelado el daemon omite `window` en el `hello` y no los escribe.

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

- Semilla de defaults en el primer arranque; presets editables/borrables; “Restore defaults” en el gestor.
- `background` es el fondo ambiental: `aurora` (el original, por defecto), `stars`, `fireflies`, `rain`, `waves`, `grid` (Sunset) o `none`. `b` cicla en idle/running/paused/finished y el daemon lo guarda; el tema solo cambia la paleta. La aurora conserva el polvo original (y marea lo comparte); estrellas, luciérnagas, lluvia, Sunset y none van sin el polvo extra. `none` no pinta ninguna celda de fondo: se ve el fondo real de la terminal (color, imagen o acrylic); los paneles, botones, barra y modales de la UI sí conservan su propio fondo. El flash de celebración se omite con `none`; el confeti sigue.
- `readable_text` (por defecto `true`) activa las bandas negras de lectura. Solo se dibujan con `none`: el texto que flota sobre la terminal (cabecera, etiqueta, meta, input, atajos, sub-hint, enlace de actualización, status/toast/error y compacto) lleva detrás una banda negra ajustada al texto con una celda de margen y tinta clara del tema. El contador no lleva banda y conserva su sombra de bloque. `n` las activa/desactiva y el daemon guarda el ajuste; con las bandas desactivadas vuelve la adaptación por OSC 11.
- `mini_timer` (por defecto `true`) activa el overlay de la barra de tareas en Windows. Solo es presentación: no cambia el timer; no viaja en el `config` de la TUI y la bandeja, el menú del widget y la tecla `M` son sus controles. En macOS no tiene efecto porque el menu bar ya muestra el tiempo.
- Sonido por defecto `digital`; volumen por defecto 80, rango 0..100 y pasos de 5 en la UI.
- `window_width`/`window_height` son el tamaño de terminal preferido en celdas y se omiten mientras no haya preferencia (0). Solo los escribe el daemon cuando la TUI reporta un resize.
- `window_position` es el borde superior izquierdo exterior de la ventana en píxeles de pantalla (admite negativos en multi-monitor) y se omite mientras no haya preferencia. Solo lo escribe el daemon cuando la TUI reporta un movimiento. Una posición fuera de rango se descarta al normalizar en vez de invalidar toda la config.
- Migración a v7 al cargar: conserva theme, sound, presets, sonido, volumen, tamaño y posición; v1 fija `sound_name=digital` y `volume=80`; v1–v4 fijan `background=aurora`; la posición queda sin definir en v1–v3; v1–v5 fijan `readable_text=true`; v1–v6 fijan `mini_timer=true`. El daemon guarda la config migrada al arrancar.
- Escritura atómica (temporal en el mismo directorio + rename). Config corrupta: backup `config.bad-<timestamp>.json` y defaults.
- Historial de sesiones: una línea JSON por countdown terminado (`started_ms`, `planned_ms`, `elapsed_ms`, `paused_ms?`, `label`, `preset_id?`, `outcome`). `outcome` es `completed` (llega a 0, también ajustando), `stopped` (stop explícito) o `replaced` (un `start` lo sustituye). No se registra un daemon que muere con el timer vivo, igual que el timer no se restaura. Cap 2000 registros con recorte atómico e histéresis (256); las líneas inválidas se saltan y un fichero ilegible se respalda como `history.bad-<timestamp>.jsonl` y arranca vacío. **No vive en `config.json`**: es un log de datos, no ajustes versionados, y no se añade un segundo store de configuración ni una base de datos.
- El daemon es el único escritor. Para el historial guarda el inicio de pared al arrancar, acumula las pausas y suma los ajustes al planificado, de modo que el registro refleja el objetivo final y el tiempo real. El cálculo de días, horas y rachas usa la zona local y un reloj inyectable en tests.
- El daemon es el único escritor. La TUI manda comandos; el daemon guarda, actualiza el menú y hace broadcast del estado compartido. El tamaño y la posición de ventana solo se guardan en Windows y se devuelven en el siguiente `hello`.
- En Windows el registro se usa solo para autostart (`HKCU\...\Run`) y para el AUMID de las notificaciones (`HKCU\Software\Classes\AppUserModelId\Juankyyy.TuickTack`, valor `DisplayName`); nunca para estado de la app. En el port macOS congelado el autostart es un LaunchAgent que apunta al binario del `.app`; el plist se reescribe al arrancar si la app se movió.
- Etiqueta y duración visibles en el menú; el orden es el almacenado (defaults primero; los nuevos al final).
- Duraciones: parser en §9. Límite 1 s..24 h.

## 9. TUI

### Pantallas y estados

- **Idle:** cabecera `READY` con `no countdown`, dígitos fantasma `00:00`, input enfocado, chips de presets y atajos de teclas bajo los presets. Mientras escribes una duración válida, los dígitos fantasma la previsualizan.
- **Running:** cabecera `RUNNING` con `ends at HH:MM`; dígitos grandes, barra, “X remaining”, botones y atajos de controles bajo los botones. Sin chips (foco).
- **Paused:** como running en ámbar, cabecera `PAUSED` con `on hold`, barra atenuada, meta con “[Space] resume”.
- **Finished / celebración:** flash + confeti + una palabra grande al azar (`TIME'S UP!`, `DONE!`, `FINISHED!`, `TIME OUT!`, `THE END!`, `BINGO!`); la cabecera muestra `TIME'S UP!` y `at HH:MM`; tras 6 s, pantalla tranquila con resumen y `r` repeat / `Enter` close. A los 10 s del finish la TUI manda `dismiss` y vuelve a idle por sí sola; si hay un modal abierto espera a que se cierre, y un `r` o un cierre manual cancelan el plazo.
- **Gestión de presets:** lista, añadir (`a`), editar (`e`), borrar (`d` con confirmación), restore defaults (`r`), start (`Enter`), close (`Esc`). `Tab` lleva el foco a la barra de acciones; `←/→` eligen acción y `Enter` la ejecuta. Las acciones que necesitan un preset se atenúan y se saltan si la lista está vacía.
- **Sonido:** modal `v` con on/off, lista Chime/Marimba/Digital/Deep, volumen 0–100 en pasos de 5, preview con `Space` y mute con `m`. Elegir sonido o cambiar volumen previsualiza si el sonido está activo.
- **Fondos ambientales:** `b` cicla `aurora` (el original), `stars`, `fireflies`, `rain`, `waves`, `grid` (visible como **Sunset**) y `none`, con toast y persistencia en config. La aurora es el default y conserva el polvo de 30 motas; marea lo comparte. Estrellas, luciérnagas, lluvia, Sunset y none no añaden polvo. `none` deja el fondo de la terminal visible: el canvas no pinta celdas vacías y la app nunca fija `BackgroundColor`/`ForegroundColor` (tampoco con los demás fondos), así que no altera el color por defecto ni el acrílico de la terminal; los elementos de UI y la sombra del contador mantienen sus fondos, y el flash de celebración se omite. Con `none`, por defecto el texto que flota directamente sobre la terminal (marca, versión, estado, sub-línea, etiqueta, meta, input, atajos, sub-hint, enlace de actualización, status/toast/error y compacto) se pinta sobre una banda negra ajustada al texto con una celda de margen y tinta clara del tema; el contador no lleva banda y conserva su sombra de bloque. `n` activa/desactiva las bandas y el daemon guarda `readable_text`. Con las bandas desactivadas, la TUI pregunta el color real del fondo por OSC 11 (`tea.RequestBackgroundColor`) al arrancar, al entrar en `none` y al conectar; si es claro, ese texto y el contador usan una tinta oscura derivada del tema con contraste WCAG (7:1 texto, 4.5:1 secundario, 3:1 grande/decorativo). Si el terminal no responde a OSC 11 o el fondo es oscuro, se conserva la paleta actual; con cualquier otro fondo no hay bandas ni adaptación. En compacto, `none` tampoco rellena el fondo: las bandas se ajustan a cada línea. `b` se ignora mientras hay texto en el input y en modo compacto. Los efectos leen el progreso, la pausa y los últimos 10 s, pero solo pintan fondo: el texto, el flash de celebración y el confeti no cambian.
- **Confirmación de cancelar:** overlay “Stop the countdown? Enter yes · Esc no”.
- **Ayuda:** overlay `?` con título "Help" y dos columnas agrupadas —Timer, Duration, Panels, Display y App— con las teclas en una columna fija, las acciones alineadas y las notas en gris; `[n]` bandas de lectura (solo con `none`) y, solo en Windows, `[M]` mini timer. Por debajo de 80 columnas usa una variante compacta de una columna; no hay scroll. Los atajos internos de presets y sonido no se duplican: cada panel los muestra al abrirse.
- **Estadísticas:** pestaña a pantalla completa con `s` (hoja sobre el escenario atenuado, como el diálogo de actualización). Cuatro secciones con `←/→`, `Tab` o `1..4`: **overview** (cabecera con el rango seleccionado; KPIs de foco, sesiones, `% of finished` —completed/(completed+stopped), las reemplazadas no penalizan— y delta contra la ventana anterior, en una o dos líneas según el ancho; barras de foco de los últimos 7 o 30 días con medio bloque, hoy en accent2 y etiquetas de día con el mes en los cambios; cabecera `records` all-time con racha actual, mejor racha —días con al menos una sesión—, día y hora más activos y sesión más larga con su etiqueta cuando cabe); **activity** (heatmap de 26 semanas con celdas de fondo, 5 niveles y etiquetas de mes/día desde `copy`; foco por hora de inicio y por día de la semana, ambos all-time; gráficas en dos columnas cuando el ancho lo permite y apiladas si no; frase de insight si queda sitio; la sección adapta leyenda, huecos e insight a la altura y puede requerir scroll a 48×22); **presets** (barras horizontales por etiqueta con duración, sesiones y `% of finished`, más el top por foco y la media); **recent** (últimas 20 sesiones con hora de inicio —`yesterday` si cabe, `yest` si no—, planificado, etiqueta y `✓ completed` / `· stopped at 4m` / `· replaced at 8m`). `t` cicla 7/30/90/all solo en el overview; el resto de secciones lo ignora y su hint no lo anuncia. El KPI usa el rango elegido; las barras son siempre la ventana diaria reciente y su note dice `last N days`, nunca el rango. `↑/↓`, `PgUp`/`PgDn`, `Home`/`End` y la rueda desplazan; `Esc` o `s` cierran. Bajo 48×22 no se dibuja y `s` se ignora; a 48×22 no hay recorte horizontal y el activity puede desplazarse en vertical. Abrir la pestaña no toca el timer; un final la cierra para enseñar la celebración, y un stop o un reemplazo desde la bandeja la dejan abierta y refrescan los datos.
- **Desconectado:** banner “Daemon disconnected — retrying…” y comandos deshabilitados.
- **Actualización disponible:** a la derecha de la marca y su versión, en la fila de la cabecera, `[Update available →]` en degradado pastel animado (arcoíris de 0,8 de vuelta a S 0,52 / L 0,68, negrita; la fase avanza ~0,1 vuelta/s con el tick de 10 Hz). Es clicable y abre el diálogo de actualización; al pasar el ratón por encima se pinta un fondo suave en toda la etiqueta. La flecha `→` es la pista estática de que abre algo. Si el estado no deja sitio, cae a `[Update →]`; si tampoco entra, no se dibuja. En `mono` usa un degradado de grises sin animación. El modo compacto añade una línea de texto sin click.
- **Buscar actualizaciones:** `u` en la pantalla principal pide una comprobación manual al daemon. Si ya hay una release conocida, `u` abre el diálogo en vez de volver a comprobar. Mientras comprueba, la línea de estado dice `Checking for updates…`; al terminar muestra un toast interno: `vX available`, `Up to date (latest vX)` o `Update check failed`. `u` funciona también en el modo compacto, pero allí no se dibuja el diálogo; la instalación requiere una ventana de al menos 48×22. La comprobación iniciada desde la bandeja o el menu bar muestra un toast nativo (WinRT en Windows, osascript en macOS; ver §12).
- **Diálogo de actualización:** modal `Update available` con la transición `vActual → vNueva` en el tono apagado del texto secundario (estática en el diálogo de elección; en progreso un brillo suave la recorre), una etiqueta `Installed · <carpeta>` o `Portable · tuick-tack.exe` (Windows) y los dos tipos en una fila compacta: la copia en uso primera y seleccionada (`› Setup · Installed` o `› Portable · In use`) y la otra al lado (`Portable · Not in use`, `Not installed` o `Not available` si falta el asset). A 44 columnas los dos chips se apilan. `Tab` cambia de copia cuando la release trae ambos assets; el hint anuncia `[Tab] type` cuando cabe. `←`/`→` mueven el foco entre la fila horizontal `install`/`download`, `github` y `later`; `Enter` ejecuta. Si el otro tipo falta, su fila muestra `Not available` y no se puede seleccionar. Al elegir la otra copia aparece una sola línea en ámbar, `This is not the copy you use; settings carry over.` (dos líneas por debajo de 50 columnas); no hay confirmación aparte. Si la release trae notas utilizables, el modal crece a 22 filas a 80×24 y a 30 filas a 120×40 y añade un panel `What's new in vNueva` con cabecera de sección en acento, viñetas, 8/16 líneas visibles y barra de scroll; `↑`/`↓` (una línea), `PgUp`/`PgDn` (una página), `Home`/`End` y la rueda desplazan las notas sin cambiar la copia ni las acciones. Sin notas el diálogo vuelve a 18 filas y no reserva el panel. El hint completo es `[Enter] select · [Tab] type · [↑/↓] notes · [Esc] later`; si no cabe, se quitan por este orden `[g] github`, las notas y `[Tab]` (`g` sigue abriendo GitHub y el botón `github` está a la vista). `g` abre GitHub, `Esc` cierra; `d`/`i` ya no existen y `←`/`→` no se anuncian como atajo. Mientras se instala, el modal pasa a progreso (`Downloading…`, `Verifying…`, `Installing…`) con barra, porcentaje y pasos `Download · Verify · Install · Reopen`. Si hay un countdown corriendo o pausado, aparece `Countdown active` con el tiempo que se perderá y `[Enter] continue` / `[Esc] back`; el daemon exige el `force` de esa segunda confirmación. El resultado del portable muestra la ruta en Descargas, `[o] open folder` y `[Enter] close`; un fallo muestra `Update failed` con `[Enter] try again`, `[g] github` y `[Esc] close`. En macOS no hay chips ni `Tab`: el diálogo muestra una sola acción `download for macOS`, el badge `Installed · <ruta>` o `Portable · Tuick-Tack.app` y los textos `macOS update` y `Unzip it and replace the current Tuick-Tack.app.`; el progreso usa `Download · Verify · Done`.

### Layout

- Alt screen, cursor oculto, mouse en modo all-motion para chips/botones y el hover del aviso de actualización.
- **Colocación de ventana recordada (solo Windows):** al conectar, si el daemon anuncia `window` con una posición guardada, la TUI mueve la ventana real del terminal antes de aplicar el tamaño y, si el host la desplaza al redimensionar, la reaplica una vez. Localiza la ventana por título exacto (`Tuick-Tack`) y clase (`ConsoleWindowClass` o `CASCADIA_HOSTING_WINDOW_CLASS`) mediante `internal/termwin`; la clase es necesaria porque en Windows Terminal el proceso solo ve un `PseudoConsoleWindow` oculto. Después, si el tamaño guardado difiere del actual, pide `CSI 8;filas;columnas t` (lo soportan conhost y Windows Terminal moderno; uno que lo ignore se queda como está). Tras aplicar el tamaño, el siguiente `WindowSizeMsg` dispara `tea.ClearScreen` para borrar el reflujo del host, que con `none` dejaría restos de la rejilla anterior. Al colocar la posición, se reaplica durante `windowPosSettle` (2 s) si el host mueve la ventana recién creada (por ejemplo, la cascada de Windows Terminal), y esa posición no se guarda.
- **Solo ventana propia:** `GetConsoleProcessList` decide si la TUI es el único cliente de su consola. Si comparte consola con un shell (p. ej. una pestaña de Windows Terminal o `go run`), no se mueve la ventana ni se guarda su posición; el tamaño sigue funcionando como antes.
- **Persistencia:** la posición se muestrea cada 500 ms desde el tick existente y se envía al daemon, junto al tamaño, con 300 ms de debounce y otra vez al cerrar con `q`/`Esc`; el `HWND` se cachea. La posición inicial de arranque, la que produce aplicar la preferencia y la cascada del host durante el asentamiento no se guardan. En reconexiones no se reaplica nada, para no pisar un resize o un movimiento del usuario. Mientras la ventana está maximizada o minimizada no se guarda ni tamaño ni posición: se conserva la última colocación normal. Si la posición guardada no cae en ningún monitor (pantalla desconectada), no se mueve y se conserva el valor.
- **macOS (congelado):** no hay colocación de ventana. El daemon omite `window` en el `hello`, la TUI no mueve ni redimensiona el terminal, `window_position`/`window_width`/`window_height` no se escriben y `configureWindow` deja los no-ops. La TUI se abre desde el menu bar con `open-tui.command`.
- Mínimo con estilos 48×22. El reloj usa escala 1 y pasa a escala 2 desde 100×30. La separación entre piezas es de una fila por debajo de 32 de alto y de dos filas desde 32 (la fila extra del aviso de actualización retrasa el cambio).
- Entre 22 y 23 filas se usa el modo *tight*: se omite la línea meta (`remaining` / `Type a duration or pick a preset`) y su hueco para que el bloque quepa sin solapes. Desde 24 filas vuelve la meta.
- Sin panel ni marco: el fondo ambiental ocupa toda la pantalla y el contenido vive en una columna centrada de máximo 100 celdas. En 80×24 quedan 4 celdas de margen a cada lado.
- Estructura vertical: cabecera de dos alturas (marca arriba; estado + sub-línea `ends at` / `on hold` / `at` / `no countdown` a la derecha) · aire · hero dígitos (7) · aire · etiqueta · aire · barra · aire · meta · aire · [input + chips | botones] · aire · atajos de teclas.
- El aviso de actualización vive en la fila de la marca, no añade filas: `● tuick·tack vX.Y.Z  [Update available]` con el estado a la derecha. La versión sigue a la marca en un tono apagado (el muted mezclado con el fondo al 85%) y cede su hueco si no deja sitio para la forma corta del aviso; el aviso se calcula contra el punto de estado y nunca lo pisa.
- Debajo de los atajos principales hay una fila en blanco y una segunda línea, más atenuada, con `[u] check for updates`, o `[u] update to vX` cuando ya hay una release conocida. En 80×24 la fila extra sale de juntar la etiqueta y la barra de progreso y de reducir el hueco entre la meta y los controles; desde 26 filas se mantiene el aire normal. En el modo tight (22–23 filas) y en compacto se omite para no quitar la línea de estado. La fila de estado sigue encima de los atajos.
- La cabecera se alinea a la medida y el bloque completo (dígitos → atajos) se centra verticalmente: los atajos quedan bajo los presets/botones, separados por el mismo aire (una fila por debajo de 30 de alto, dos desde 30), y el espacio sobrante queda bajo ellos. En 48×22 y 80×24 el bloque llena justo el alto disponible. La línea de estado (desconectado / aviso) se dibuja en la fila inmediatamente superior a los atajos.
- Por debajo de 48×22 (anchura < 48 o altura < 22) la TUI pasa a modo texto compacto: fondo plano, sin fondo ambiental, polvo, confeti, bigfont, chips ni botones. Muestra por prioridad el reloj, la etiqueta, el estado y los atajos esenciales, y recorta las líneas de menor prioridad hasta dejar solo el reloj en una fila. Los modales de gestión no se dibujan y sus teclas se ignoran; la confirmación de parada se muestra como texto. Al recuperar el mínimo se restaura la vista anterior.
- Los modales se dibujan sobre un scrim que atenúa el escenario y una hoja plana sin marco, con el mismo contenido y botones.

### Teclas

Las pistas de la TUI usan el formato `[tecla] acción` (por ejemplo, `[Space] pause`); los botones conservan sus propios corchetes.

| Contexto | Acción |
|---|---|
| Idle | dígitos/`:`/`h`/`m`/`s` escriben; `Backspace` borra; `Enter` inicia; `←/→` seleccionan preset; `p` gestor; `t` tema; `v` sonido; `m` mute; `s` estadísticas; `M` mini timer; `u` busca actualizaciones; `?` ayuda; `q` salir de la TUI |
| Running | `Space` pausa; `Esc` confirma cancelar; `+`/`-` ±1 min; `v` sonido; `m` mute; `s` estadísticas; `M` mini timer; `u` busca actualizaciones; `?` ayuda; `q` salir |
| Paused | `Space` reanuda; `Esc` confirma cancelar; `+`/`-` ±1 min; `v` sonido; `m` mute; `s` estadísticas; `M` mini timer; `u` busca actualizaciones; `?` ayuda |
| Finished | `Enter`/`Esc` cierran; `r` repiten; `v` sonido; `m` mute; `s` estadísticas; `M` mini timer; `u` busca actualizaciones |
| Estadísticas | `←/→`/`Tab` sección; `1..4` directo; `t` rango; `↑/↓` `PgUp/PgDn` `Home/End` rueda scroll; `Esc`/`s` cerrar |
| Sonido | `↑/↓` sonido; `←/→` volumen ±5; `Space` preview; `m` mute; `Esc`/`Enter` cerrar |
| Gestor | `↑/↓` mover; `Tab` acciones; `←/→` elegir acción; `Enter` iniciar (lista) o ejecutar (acción); `a` añadir; `e` editar; `d` borrar; `r` defaults; `Esc` cerrar; en formulario `Tab` campo, `Enter` guardar, `Esc` cancelar |
| Actualización | `←/→` acción; `↑/↓` `PgUp`/`PgDn` `Home`/`End` rueda notas; `Enter` ejecutar; `g` GitHub; `Esc` cerrar. En Windows, `Tab` cambia Setup/Portable; en el port macOS congelado hay una sola acción `download for macOS` |

`M` (mini timer) solo existe en Windows; en macOS el overlay no existe, así que la tecla y la fila de Help no se muestran.

### Parser de duraciones

| Entrada | Interpretación |
|---|---|
| `25` | 25 minutos |
| `25m` | 25 minutos |
| `90s` | 90 segundos |
| `1h30m` | 1 h 30 min |
| `12:30` | 12 min 30 s |
| `1:02:03` | 1 h 2 min 3 s |

- Sin decimales; `,` y `.` no son separadores.
- Clamp 1 s..24 h; error inline si no es válido.
- Formato de display: `MM:SS` bajo 1 h; `H:MM:SS` desde 1 h; segundos restantes redondeados hacia arriba.
- Hora de fin en formato local 24 h.

### Sincronización

- Snapshot a 10 Hz; la TUI interpola `remaining` con su propio reloj entre snapshots y corrige al recibir el siguiente.
- `finish_seq` evita celebrar dos veces. Si la TUI conecta con el timer ya en `finished`, celebra una ráfaga corta.
- Si llega el final con un modal abierto, se cierra el modal y se celebra.
- El cierre de la TUI no toca el daemon. Al salir se restaura la terminal y se imprime “The countdown is still running in the tray.” en Windows o “The countdown is still running in the menu bar.” en macOS.
- Si el daemon se cierra desde la bandeja o el menu bar, envía `daemon_closing`, la TUI sale sin reintentar y sin relanzar el daemon, e imprime “The daemon quit; closing.” en vez del aviso del timer.

## 10. Dirección visual

### Paletas

**Midnight (default)**

| Token | Hex |
|---|---|
| bg / bg2 | `#070B14` / `#0B1524` |
| panel | `#0C1524` |
| border | `#1E3150` |
| text / muted | `#E7ECF5` / `#6C7A96` |
| accent / accent2 | `#6EE7B7` / `#60A5FA` |
| warn / danger | `#FBBF24` / `#F87171` |
| confeti | `#6EE7B7` `#60A5FA` `#A78BFA` `#F472B6` `#FBBF24` `#F8FAFC` |

**Ember:** bg `#120C0A`, panel `#1A1210`, border `#3A2520`, accent `#FFB86B`, accent2 `#FF7A59`, text `#F7EDE8`, muted `#8C7A72`, confeti cálido.
**Mono:** bg `#0A0A0C`, panel `#121216`, border `#2A2A33`, accent `#E5E7EB`, accent2 `#9CA3AF`, text `#F4F4F5`, muted `#71717A`, confeti grises/blanco.

### Elementos

- **Fondos ambientales:** `aurora` es el degradado vertical + dos halos radiales original; `stars` es un campo en perspectiva con punto de fuga centrado: las estrellas salen hacia los bordes con estelas de puntos (cabeza `+`/`*`), arranca con 80 y va revelando hasta 320 al 100% del progreso, con la misma velocidad de cabeza y estelas que crecen con el progreso y el pulso de los últimos 10 s, más una fugaz ocasional; `fireflies` es un enjambre de 14-18 luces grandes, cálidas y con halo 3×3 que flotan en la mitad inferior; `rain` cae en columnas de glifos ASCII; `waves` pinta tres bandas sinusoidales desde abajo; `grid` (visible como **Sunset**) dibuja un sol retro ámbar semienterrado con cortes, halo, reflejo y una rejilla en perspectiva; `none` no pinta el fondo y deja ver la terminal. Todos usan los colores del tema, se mueven a 10 Hz, se atenúan al pausar y en la celebración, y mantienen el contraste por debajo del texto (el aro del sol es la única excepción deliberada). El flash es global salvo con `none`; el polvo se suma solo a aurora y marea.
- **Escenario abierto:** sin panel, sin marco y sin borde. El contenido flota directamente sobre el fondo ambiental en una columna centrada de hasta 100 celdas; no hay caja que simule una ventana.
- **Cabecera de dos alturas:** marca y dot a la izquierda; estado en mayúsculas y dot a la derecha; sub-línea derecha con `ends at HH:MM`, `on hold`, `at HH:MM` o `no countdown`. Sin píldora de estado.
- **Dígitos:** fuente de bloques 7×5, escala horizontal 1 en compacto y 2 en amplio, degradado por fila, sombra de 1 celda (abajo-derecha) y halo tenue; se mantiene también con `none`, con la sombra en un gris claro cuando el fondo de la terminal es claro. Las palabras de celebración usan la misma fuente y la escala 2 desde 100 columnas con espaciado proporcional (recorta los huecos laterales de cada glifo) para que las frases largas quepan sin volver a 1×; los dígitos siguen monoespaciados para que el reloj no cambie de ancho.
- **Barra de progreso:** 1 fila, relleno con degradado accent→accent2, pista oscura, cabeza blanca brillante y checkpoints `┆` cada 25%.
- **Detalles vivos:** colon parpadeante, dot de estado pulsante, ~30 motas de polvo lentas, pulso ámbar en los últimos 10 s.
- **Sin emoji ni runas de doble ancho.** Todo el contenido usa glifos de una celda y box-drawing.
- **Truecolor**; Windows Terminal recomendado. `conhost` moderno también soporta 24-bit.

### Anti-patrones

- No llenar la pantalla de cajas y bordes.
- No simular una ventana: ni panel centrado, ni marco redondeado, ni píldora de estado.
- No cambiar el color del texto completo por estado; solo acentos, barra y cifras.
- No animar a 60 fps el idle: 10 Hz para polvo/estado/fondos; 30 fps solo en la celebración.

## 11. Celebración y confeti

### Secuencia

1. `finish_seq` cambia → flash de fondo (~120 ms) + arranca el sonido del daemon.
2. Ráfagas de confeti + una de seis palabras en bloques grandes (`TIME'S UP!`, `DONE!`, `FINISHED!`, `TIME OUT!`, `THE END!`, `BINGO!`), elegida al azar una vez por `finish_seq` y sin repetir la del finish anterior.
3. ~6 s de animación; el sonido dura entre 1,7 y 4 s según el pack.
4. Pantalla de celebración tranquila: “5 min completed”, hora, `r` repeat, `Enter` close. A los 10 s del finish la TUI manda `dismiss` y vuelve a idle por sí sola, salvo que haya un modal abierto (espera a que se cierre).
5. `Enter`/`Esc` → idle y parar sonido; `r` → repetir la misma duración y volver al countdown.

### Confeti

- ~110 partículas base; la cifra escala por área hasta ~160 en 120×40, con tope 240.
- Ráfagas: popper central (32) a t=0, cañones izquierdo y derecho (15 cada uno) a 0.12/0.26 s, lluvia superior (35) a 0.55 s y segunda oleada (13) a 2.4 s. En una TUI que conecta con el temporizador ya terminado, una ráfaga corta de ~52.
- Física de papel: gravedad 11–16 filas/s² escalada, rozamiento dependiente de la orientación (de canto cae rápido; plano frena), caída terminal ~3–7 filas/s, deriva sinusoidal, bob vertical, giro 2.5–6 rad/s y `dt` limitado a 50 ms.
- Cada papel es una ficha de 2×1 celdas: cuerpo plano del color de la paleta, brillo `▘` arriba a la izquierda y sombra `▄` abajo a la derecha, más una sombra proyectada `▀` una celda abajo-derecha. Solo glifos de bloque `█ ▀ ▄ ▌ ▘`; nada de dithers, rombos, destellos tipográficos, emoji ni runas de doble ancho.
- Cuatro frames de giro derivados del ángulo: ficha completa, media ficha superior `▀▀`, canto `▌` y media ficha inferior `▄▄`. Los destellos son una cruz de 5 bloques con centro blanco.
- Sin mezcla continua con el fondo: el fade quita celdas (primero la sombra proyectada y la mitad derecha, luego la izquierda) en el último 15% de vida y en el fundido global de 5.0 a 5.7 s.
- Al redimensionar, las posiciones se remapean proporcionalmente; las piezas que salen de pantalla se recortan.
- Dibujo por capas: fondo → polvo → sombras proyectadas → fichas → destellos → texto. La palabra y el resumen se dibujan encima para que el confeti pase por detrás.
- 30 fps con `tea.Every(33ms, …)` durante la celebración; después vuelve el tick de 100 ms.
- RNG con semilla en tests; en producción semilla temporal. La palabra de celebración se deriva de `seed + finish_seq`, se fija al empezar el finish y se mantiene hasta el siguiente.

## 12. Bandeja (menu bar macOS congelado)

> El menu bar de macOS pertenece al port congelado: no se compila en el flujo normal y solo se valida en un build manual.

`systray.Run(onReady, onExit)` ocupa el hilo principal; engine + IPC viven en goroutines. La API es global y segura desde goroutines. En Windows es la bandeja del sistema (ICO); en macOS (congelado) es el menu bar (PNG 32 px con cgo). Sin cgo, macOS compila un tray no-op que mantiene la API del daemon.

### Menú

```
Tuick-Tack · 04:32 remaining
Update available: v0.2.0
Check for updates
──────────────────────────
Open Tuick-Tack
Start              ▸ 5 min · 10 min · 15 min · Custom…
Pause / Resume
Stop
Sound              ✓
Sound settings     ▸ Chime · Marimba · Digital · Deep · Preview
Volume 80%         ▸ -5% · +5% · 0% · 25% · 50% · 75% · 100%
Start with Windows / Start at login
Mini timer         ✓
──────────────────────────
Quit
```

- **Clic izquierdo (Windows):** abre la TUI (`SetOnTapped`).
- **Clic derecho (Windows):** menú. No se define `SetOnSecondaryTapped` para que systray muestre el menú.
- **Clic en macOS:** abre el menú; la TUI se abre con el ítem **Open Tuick-Tack** (no se usa `SetOnTapped`).
- **Status:** ítem deshabilitado con el tiempo restante; se actualiza a 1 Hz mientras corre. En macOS el tiempo se pinta junto al icono del menu bar.
- **Update available:** aparece solo cuando el daemon conoce una release más nueva; `Update available: vX.Y.Z`, habilitado y abre la release en el navegador. Se crea oculto justo después del status, así que al mostrarse conserva esa posición. No admite color: es un menú nativo.
- **Check for updates:** siempre visible. Pide una comprobación manual al daemon, muestra `Checking for updates…` y se deshabilita mientras corre, y luego queda como `Up to date (vX)` o `Update check failed` (se puede volver a pulsar). Cada comprobación desde aquí lanza además un toast nativo con el resultado (WinRT en Windows, osascript en macOS); en Windows el de una release nueva abre la página al pulsar el cuerpo, en macOS no hay clic.
- **Start:** submenú reconstruido cuando cambian los presets; “Custom…” abre la TUI con el input enfocado.
- **Pause / Resume:** un solo ítem con título dinámico; habilitado según estado.
- **Stop:** habilitado en running/paused/finished; en finished actúa como dismiss.
- **Sound:** check; persiste en config y silencia el sonido de fin.
- **Sound settings:** submenú con los cuatro sonidos y check en el activo; Preview reproduce la selección. Elegir un sonido previsualiza si Sound está activo.
- **Volume:** submenú con título dinámico; -5%/+5% y atajos 0/25/50/75/100. Cada cambio previsualiza si Sound está activo.
- **Start with Windows / Start at login:** check. En Windows lee/escribe `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, valor `TuickTack` apuntando a `"tuick-tack.exe" --daemon --hidden`; sin admin. Al arrancar, el daemon migra un valor antiguo de `tuick-tack-tray.exe` para conservar el ajuste. En macOS crea/borra `~/Library/LaunchAgents/com.juankyyy.tuick-tack.plist` con `RunAtLoad=true` y `KeepAlive=false`, apuntando al binario del `.app` con `--daemon --hidden`; `launchctl bootstrap/bootout gui/$(id -u)` aplica el cambio y el plist se reescribe al arrancar si la app se movió. En builds de `go run` el autostart queda deshabilitado: la ruta del ejecutable es temporal.
- **Mini timer:** check solo en Windows. Activa el overlay de la barra de tareas y persiste en `mini_timer`; por defecto está activado. En macOS el ítem no se crea: el menu bar ya muestra el tiempo junto al icono.
- **Quit:** cierra el loop y llama a `systray.Quit()`; avisa `daemon_closing` a las TUIs conectadas antes de cerrar el pipe/socket, para que la TUI se cierre en vez de relanzar un daemon. Después para el sonido.

### Mini timer (solo Windows)

Windows 11 no permite incrustar widgets de terceros en la barra de tareas: los deskbands desaparecieron y el panel de Widgets exige MSIX e identidad de paquete. El mini timer es una **ventana propia**, no una integración nativa: se superpone al hueco izquierdo de `Shell_TrayWnd` y no inyecta nada en Explorer.

- **Ventana:** `WS_POPUP` con `WS_EX_TOOLWINDOW|WS_EX_TOPMOST|WS_EX_NOACTIVATE`, sin entrada en Alt+Tab ni en la barra, mostrada con `SW_SHOWNOACTIVATE`. Tiene su propio hilo y message loop (`runtime.LockOSThread`), separado del loop de `systray`, y se destruye al cerrar el daemon.
- **Posición:** barra horizontal (inferior o superior) a 8 px del borde izquierdo y centrada en vertical. A 100 % la base mide 128×30 px: rejilla de 42×9 celdas de 3 px, borde de 1 px y barra de 108×3. El ancho crece a 143 px cuando el tiempo es `10:00:00`–`24:00:00` (47 celdas); la barra sigue centrada y no se estira. A 125 % la celda es de 4 px y a 150 % de 5 px; todo escala en bloques enteros (sin interpolación).
- **Contenido:** solo el tiempo restante en el bigfont 5×7 de la TUI (`MM:SS` o `H:MM:SS`), sombra de una celda y la barra de progreso; sin etiqueta, transcurrido ni texto de estado.
- **Colores:** fondo `Panel`, borde `Border`, dígitos `Accent → Accent2`, sombra `Scale(BG, 0.5)`, track `Lerp(Panel, BG, 0.35)`, relleno `Accent → Accent2` y cabeza `Text`. En pausa, dígitos y barra en `Warn`. Al terminar, `00:00`, barra llena y dígitos `Warn → Danger`. En los últimos 10 s el relleno puede pulsar como el TUI. El tema se lee del config y un cambio repinta en el siguiente tick.
- **Visibilidad:** solo si `mini_timer` está activo, hay sesión `running`, `paused` o `finished`, la barra está visible y horizontal, no hay fullscreen ni un flyout del shell delante y la ventana de la TUI no tiene el foco. En idle se oculta; en finished queda visible hasta dismiss/stop.
- **Foco:** cada 500 ms comprueba el título y la clase del foreground (`GetForegroundWindow` + `GetWindowTextW`/`GetClassName`). Si es `Tuick-Tack` con clase `ConsoleWindowClass` o `CASCADIA_HOSTING_WINDOW_CLASS`, la TUI tiene el foco y el widget se oculta; si está detrás, sigue visible. No usa `EnumWindows` en el poll, para no agotar la tabla de callbacks de Go en un daemon de larga vida. Una pestaña inactiva de Windows Terminal no lo oculta.
- **Auto-hide y fullscreen:** con la barra auto-oculta, el widget solo aparece cuando el cursor está sobre la barra; `SHQueryUserNotificationState` oculta además en fullscreen, presentación, pantalla bloqueada y modo silencioso. Si Explorer se reinicia, `Shell_TrayWnd` se vuelve a localizar; si la barra es vertical o secundaria, no se dibuja la variante interior. Los flyouts se detectan por clase y, para `Windows.UI.Core.CoreWindow`, solo se ocultan si el proceso es del shell (`explorer.exe`, `StartMenuExperienceHost.exe`, `SearchHost.exe`, etc.), no por cualquier app UWP.
- **Interacción:** clic izquierdo abre o enfoca la TUI y oculta el widget en el acto; clic derecho abre un menú nativo con **Open**, **Pause/Resume**, **Stop** y **Hide mini timer**. Hide desactiva `mini_timer` y persiste. La tecla `M` de la TUI, en las pantallas principales (idle, running, paused y finished), envía `toggle_mini_timer` y hace lo mismo sin abrir la bandeja. No hay arrastre.
- **Configuración:** `mini_timer` en el esquema 7, por defecto `true`; el daemon es el único escritor. Es presentación local: la TUI solo lo alterna con `toggle_mini_timer` (aditivo, sin subir `Protocol`), no recibe su estado y no lo edita directamente.
- **Límites:** puede tapar el botón de Widgets/tiempo si está en ese hueco; no se mueve automáticamente. No cubre barras verticales ni las secundarias de multi-monitor. Es una ventana topmost, no un widget del shell.

### Actualizaciones

- El daemon comprueba `https://api.github.com/repos/Juankyyy/tuick-tack/releases/latest` al arrancar (en segundo plano, sin bloquear tray/IPC) y después cada 24 h. Si falla (offline, proxy, 403 por rate limit), reintenta a las 6 h; solo queda en el log, sin toast.
- La comprobación manual se pide desde la bandeja (**Check for updates**) o desde la TUI con `u`. El daemon la ejecuta en una goroutine (las comprobaciones manuales y automáticas se serializan), actualiza la caché y responde `update_result` al cliente que la pidió; el estado de la bandeja pasa a `Checking…`, `Up to date (vX)` o `Update check failed`.
- La comprobación iniciada desde la bandeja/menu bar muestra un toast nativo con el resultado: `Update available` (con la versión), `You're up to date` (con la versión actual) o `Update check failed`. En Windows usa `Windows.UI.Notifications` vía `github.com/mtunjic/winrt-toast` v1.2.0 (COM puro, sin cgo ni PowerShell) y el toast de una release nueva abre la página al pulsarlo. En macOS usa `osascript -e 'display notification ...'`; no hay clic. Las comprobaciones automáticas y las iniciadas desde la TUI no muestran toast del sistema; la TUI ya tiene su propio aviso.
- En Windows el toast usa el AUMID `Juankyyy.TuickTack`, registrado una vez en `HKCU\Software\Classes\AppUserModelId` con `DisplayName=Tuick-Tack`. No requiere admin ni acceso directo en el menú Inicio para mostrarse; el `CustomActivator` solo haría falta para recibir callbacks, que no se usan (los toasts con release usan activación por protocolo). Focus Assist o las notificaciones desactivadas para la app pueden ocultarlo; el estado de la bandeja sigue siendo la fuente de verdad. En macOS las notificaciones las muestra el sistema y el usuario puede desactivarlas en Ajustes.
- Solo builds de release: `buildinfo.Version == "dev"` no comprueba automáticamente; la comprobación manual sí funciona en `dev`, pero `install_update` y `download_portable` se rechazan en builds `dev` porque el ejecutable es temporal. `tuick-tack --check-updates` hace una pasada y la imprime.
- Comparación semver propia (`internal/update`): acepta `v`, ignora metadata de build, pre-release antes que release; `dev` ordena antes que cualquier versión.
- Las releases de Windows exponen los assets estables `tuick-tack-setup.exe`, `tuick-tack.exe` y `SHA256SUMS.txt`; una release macOS puntual expone `tuick-tack-macos-universal.zip` + `.sha256`. `internal/update` los localiza por nombre y calcula el SHA-256 con el `digest` del asset o con el fichero de checksums de la plataforma. Si faltan, la acción correspondiente queda deshabilitada y solo se ofrece la página de la release.
- El resultado se cachea en memoria (con los assets y el `body` de la release), viaja en el `hello` y se emite como `update` cuando aparece. La TUI lo revalida contra su propia versión.
- **Notas de la release:** `internal/update` lee `body` del JSON, normaliza los saltos de línea y lo recorta a 16 KiB por límite de rune. La TUI muestra la sección entre el primer `##` que menciona `What's new` y el siguiente `##`; si no existe, cae al body completo. El renderizado (`internal/tui/notes`) quita Markdown, HTML, entidades, ANSI, emojis y cualquier runa que no sea de una celda, y envuelve a 56/34 columnas con sangría colgante.
- **Instalar update (Windows):** el daemon descarga el setup en `%LOCALAPPDATA%\tuick-tack\updates`, verifica el SHA-256 y exige `force` si hay un countdown corriendo o pausado. Después copia el ejecutable actual a un helper temporal, lo arranca oculto y cierra el daemon con `daemon_closing{reason:"update"}`. El helper espera a que el daemon y la TUI liberen sus mutex, ejecuta el setup con `/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CLOSEAPPLICATIONS`, busca la copia instalada (entrada de desinstalación de Inno) y reabre la TUI. Si el setup falla, relanza la copia anterior y muestra un toast. En el port macOS congelado `install_update` responde `unsupported` y la TUI no lo ofrece.
- **Descargar portable (Windows):** el daemon descarga `tuick-tack.exe`, verifica el SHA-256 y lo guarda como `tuick-tack-vX.Y.Z.exe` en la carpeta Downloads del usuario (`FOLDERID_Downloads`). No reinicia ni cierra nada; la TUI muestra la ruta y ofrece abrirla en Explorer.
- **Descargar para macOS (port congelado):** el daemon descarga `tuick-tack-macos-universal.zip`, verifica el SHA-256 (digest del asset o `.sha256`), lo guarda como `tuick-tack-vX.Y.Z-macos-universal.zip` en `~/Downloads` y lo revela con `open -R`. No reinicia ni sustituye el `.app`; la TUI muestra la ruta y recuerda descomprimir y reemplazar `Tuick-Tack.app`.
- Privacidad: una petición HTTPS anónima por comprobación, `User-Agent: tuick-tack/<versión>`, sin telemetría propia. Las descargas solo ocurren tras una confirmación explícita.
- El setup es por usuario, sin admin. Instala en `{userdocs}\tuick-tack`, crea el acceso al menú Inicio (y opcionalmente al escritorio) y registra el desinstalador; conserva la configuración y el autostart existente. La primera instalación manual debe hacerse con la app vieja cerrada (o el setup pedirá cerrarla); las siguientes actualizaciones desde la TUI ya coordinan el cierre. Sigue sin firma de código. En el port macOS congelado no hay setup: la app se distribuye como `.app` dentro del zip, firmada ad-hoc y sin notarizar; el usuario la mueve a `~/Applications` o `/Applications`.

### Iconos

- Iconos de bandeja/menu bar de píxeles duros, rejilla 7×7 con una celda de margen y sin antialiasing; estados play, agujas, pausa y 0 en el color del estado.
- Windows: 4 ICO embebidos (`idle`, `running`, `paused`, `finished`) con 16/20/24/32/40/48/56/64 px. `systray` carga el HICON de 2× (32/40/48/56/64) y el shell lo reduce a 16/20/24/28/32; la rejilla y el margen están pensados para esa reducción.
- macOS (congelado): 4 PNG de 32×32 (`assets/icons/idle.png`, etc.) con la misma rejilla. El fork de Fyne fija el tamaño a 16 pt; el PNG de 32 px se ve nítido en Retina.
- El `.ico` del ejecutable (`assets/icon.ico`) es una loseta navy de 16×16 celdas con anillo mint y agujas blancas, con 16/24/32/40/48/64/96/128/256 px. `winres` lo embebe completo en el `.syso`; `assets/icon.png` es el máster de 256 px para previews.
- No se regenera el icono cada segundo: systray escribe cada icono nuevo en un archivo temporal. El tiempo va en el tooltip.
- Tooltip: `Tuick-Tack · ready`, `Tuick-Tack · 04:31 remaining (5 min)`, `Tuick-Tack · time's up!`.
- Actualización de título/estado/habilitados en cada snapshot (1 Hz) y reconstrucción del submenú de presets solo al cambiar config.

### Abrir la TUI

- El tray lanza `os.Executable()` (el propio `tuick-tack.exe`). Si Windows Terminal es el terminal por defecto (`HKCU\Console\%%Startup`, CLSID estable o Preview), `internal/wt` lee **solo** el `defaultProfile` de su `settings.json` (estable/Preview, empaquetado/desempaquetado, tolerante a JSONC) y abre una ventana nueva con `wt -w -1 new-tab --useApplicationTitle -p <perfil> <exe> --launch-handled`. Así la TUI hereda el aspecto real del perfil (acrílico, opacidad, imagen, esquema) sin escribir nada en la configuración de la terminal. `--useApplicationTitle` deja que la TUI fije el título `Tuick-Tack` que `termwin` necesita.
- Si Windows Terminal no es el terminal por defecto, no está instalado, su alias está desactivado o el perfil no se puede leer, el lanzamiento cae a `windows.CreateProcess` con `CREATE_NEW_CONSOLE` (comportamiento anterior): la app nunca deja al usuario sin TUI ni cambia su terminal por su cuenta. El fallback de consola usa `CreateProcess` y no `os/exec` porque este último conecta stdin/stdout/stderr a `NUL` y activa `STARTF_USESTDHANDLES`, dejando la consola nueva vacía. `wt` sí usa `os/exec`/`winproc.StartHidden` porque es una app GUI: no necesita consola.
- **Acceso directo, Explorer o instalador:** el proceso arranca en una ventana que Windows Terminal crea con el perfil **Defaults** (sin el acrílico del usuario). Antes de adquirir el mutex de la TUI, `RelaunchTUI` comprueba que la TUI sea dueña de su consola (`termwin.OwnsConsole`), que no venga ya marcada con `--launch-handled` y que WT sea el terminal por defecto; si todo encaja, lanza la ventana con `wt -p` y termina. La ventana inicial se cierra y el parpadeo es breve. En una pestaña compartida con un shell no se relanza: ya tiene el perfil del usuario. Si el lanzamiento con WT falla, sigue en la ventana actual.
- La comprobación de instancia es la misma en ambos caminos: `TUIRunning` antes de lanzar y el mutex `<pipe>-tui-mutex` al arrancar. Una segunda apertura enfoca la TUI existente y no crea otra.
- La TUI mantiene el mutex `<pipe>-tui-mutex`; una segunda instancia llama a `FocusTUI()` y sale.
- `FocusTUI` busca una ventana visible con título exacto `Tuick-Tack` y clase `ConsoleWindowClass` o `CASCADIA_HOSTING_WINDOW_CLASS`, la restaura si está minimizada, suelta el foreground lock con un toque de Alt y usa `SetForegroundWindow`; si falla, `SwitchToThisWindow`. El título exacto y la clase evitan coincidir con editores o con OpenChamber.
- Limitación: si la TUI está en una pestaña no activa de Windows Terminal, el título de la ventana no coincide; no se duplica, pero puede no enfocarse.
- **macOS (congelado):** `Open Tuick-Tack` en el menu bar escribe `~/Library/Application Support/tuick-tack/open-tui.command` (chmod 0700) y lo abre con `open`; el script hace `exec` del binario real para que la TUI herede la terminal del usuario. Esto evita el permiso TCC de AppleScript. No se intenta localizar la ventana de Terminal; una segunda TUI activa Terminal con `open -a` como best effort y sale.
- No hay búsqueda de un segundo binario: TUI y daemon son el mismo archivo.

## 13. Sonido

- Pack de cuatro WAV originales, 44.1 kHz, 16-bit, mono, generado por `tools/gen-sounds` (sin samples con copyright):
  - `chime`: arpegio de campana de 4.0 s.
  - `marimba`: figura de madera de 2.8 s.
  - `digital`: motivo synth de 1.7 s (predeterminado).
  - `deep`: gong grave de 4.0 s.
- Los WAV viven en `assets/sounds/` y se embeben con `//go:embed`; `go generate ./assets` los regenera de forma determinista.
- Al preparar la reproducción, el daemon escala las muestras PCM por `volume/100` y escribe una variante en `%LOCALAPPDATA%\tuick-tack\sounds\<sonido>-<volumen>.wav` en Windows o `~/Library/Caches/tuick-tack/sounds/<sonido>-<volumen>.wav` en macOS (escritura atómica, limpieza de variantes antiguas).
- Reproducción Windows: `winmm.dll!PlaySoundW(path, 0, SND_FILENAME|SND_ASYNC|SND_NODEFAULT)`.
- Reproducción macOS: `afplay <ruta>` con el mismo WAV escalado; el proceso se mata al parar, cambiar de sonido o volumen, o al dismiss.
- Parar: `PlaySoundW(nil, 0, 0)` en Windows y matar `afplay` en macOS, al dismiss, repetir, nuevo start, cerrar el modal o cambiar de sonido/volumen.
- Volumen 0 no reproduce nada. `sound=false` silencia el fin; el preview explícito funciona salvo a volumen 0.
- Elegir sonido o cambiar volumen desde la TUI o la bandeja/menu bar previsualiza si `sound=true`; el preview no altera el volumen del sistema.
- Fallo de caché o audio → log y seguir; la bandeja/menu bar y la TUI avisan igual.
- `SND_MEMORY` no se usa (no combina con `SND_ASYNC`); se usa archivo.

## 14. Render por celdas

- `internal/tui/canvas`: rejilla de celdas `{ch rune; fg, bg color.RGBA; bold bool}` con `Set`, `Text`, `Box`, `Fill`, `Gradient`, `Blit`, `Center`, `Dim` (scrim de modales, solo sobre colores presentes) y `Render() string` (ANSI SGR 24-bit, reset por fila, sin runas anchas). `Render` emite solo los atributos que existen: una celda sin `FG`/`BG` conserva el color real de la terminal, lo que hace posible el fondo `none`.
- `internal/tui/bigfont`: fuente 7×5 para `0-9`, `A-Z` y `: ' ! . - ?`; dibuja con gradiente por fila y sombra, y tiene un modo proporcional para las palabras de celebración (recorta los huecos laterales; los dígitos siguen monoespaciados).
- `internal/tui/theme`: paletas y utilidades de mezcla/lerp, incluidos `Luminance`, `Contrast` e `Ink` para adaptar la tinta al fondo de la terminal.
- `internal/tui/stage.go`: bandas de lectura negras (`band`, `drawCenteredBand`) y selección de color de escenario (`stageColor`), activas solo con `none` y `readable_text`.
- `internal/tui/notes`: extrae la sección `What's new` del body de la release, quita Markdown/HTML/ANSI/emoji y envuelve el texto a celdas; el resultado lo pinta el diálogo de actualización con scroll.
- `internal/tui/fx`: `Backdrop` y sus fondos (`aurora`, `stars`, `fireflies`, `rain`, `waves`, `grid`/Sunset, `none`), el polvo ambiental y el sistema de partículas del confeti. `Draw` es una función del tiempo y de una semilla, sin estado acumulado, para que pausar, redimensionar o redibujar no desincronice el efecto.
- `View()` de Bubble Tea devuelve `tea.NewView(canvas.Render())`; alt screen y mouse se declaran según la API v2.
- Se redibuja solo cuando hay cambios o tick: 10 Hz normal, 30 fps en celebración, 10 Hz para el polvo.
- Hot loop sin allocations por celda (buffers reutilizados); frame < 8 ms a 120×40.
- Tests de ancho: ninguna línea supera el ancho; todas las runas son de una celda.

## 15. Estructura del proyecto

```
go.mod
cmd/
  tuick-tack/main.go          # TUI sin args; daemon con --daemon
  tuick-tack/rsrc_windows_amd64.syso
internal/
  config/                     # load/save/validate, defaults
  duration/                   # parser
  engine/                     # timer state machine + tests
  protocol/                   # mensajes serde
  ipc/                        # server/client: named pipe (Windows), Unix socket (macOS, congelado)
  stats/                      # historial JSONL + agregados de la pestaña
  daemon/                     # wiring, sound, autostart, logging, update orchestration + hojas _windows/_darwin (macOS congelado)
  minitimer/                  # overlay de la barra de tareas (Windows); no-op en macOS
  update/                     # release check, assets por GOOS, download + SHA-256
  updater/                    # helper de Windows que ejecuta el setup (no-op en macOS congelado)
  install/                    # registro de Inno en Windows; detección de bundle .app en macOS congelado
  instance/                   # mutex (Windows) / flock (macOS congelado) de daemon y TUI
  winproc/                    # CreateProcess oculto (Windows) y Setsid (macOS congelado)
  wt/                         # lee el perfil por defecto de Windows Terminal y abre la ventana
  termwin/                    # localizar y mover la ventana real de la consola (Windows; no-op en macOS congelado)
  explorer/                   # abrir Downloads con el fichero seleccionado (Explorer; Finder en el port congelado)
  browser/                    # ShellExecuteW / open (macOS congelado)
  notify/                     # toasts WinRT (Windows) y osascript (macOS congelado)
  tray/                       # systray menu/menu bar, iconos, tooltip; no-op sin cgo (macOS congelado)
  tui/
    app.go                    # Bubble Tea model/update
    window.go                 # tamaño y posición de la ventana recordados
    view.go                   # pantallas y layout
    stats.go                  # pestaña de estadísticas y sus gráficas
    copy/                     # todo el copy de la UI
    notes/                    # sección What's new -> líneas de una celda
    canvas/
    bigfont/
    theme/
    fx/                       # backdrops + dust + confetti
assets/
  sounds/ chime.wav marimba.wav digital.wav deep.wav
  icon.ico                    # ICO multi-tamaño del ejecutable (winres)
  icon.png                    # máster 256 px para previews
  icon.iconset/               # entradas del .icns de macOS (congelado)
  icons/ idle.ico running.ico paused.ico finished.ico
  icons/ idle.png running.png paused.png finished.png   # menu bar de macOS (32 px, congelado)
  banner.txt                  # ASCII 2x del nombre (branding, no se embebe)
  banner-compact.txt          # ASCII 1x para terminales de 80
  banner.png                  # PNG transparente, celdas cuadradas
tools/
  gen-sounds/main.go
  gen-icons/main.go
  gen-banner/main.go          # banner desde el 5×7 de bigfont
  gen-installer/main.go       # imágenes de marca del asistente de Inno
installer/
  tuick-tack.iss              # setup por usuario (Inno Setup 6)
  wizard-large.png            # panel de marca 164×314
  wizard-small.png            # cabecera 55×55
macos/
  Info.plist                  # bundle, LSUIElement, versión (congelado)
  launcher                    # wrapper que arranca el daemon (congelado)
  README.txt                  # Gatekeeper, datos y upgrade (congelado)
scripts/
  build.ps1                   # build de release de Windows
  build-macos.sh              # build cgo + .app universal + zip + .sha256 (congelado, manual)
winres/
  winres.json                 # icono, manifiesto y versión del ejecutable
  generate.go                 # directiva go:generate del .syso
.github/workflows/
  ci.yml                      # Windows en push/PR; job macOS solo manual
  macos.yml                   # build/paquete/adjunto macOS, solo workflow_dispatch
```

Módulo Go provisional: `tuick-tack` (local). Se puede cambiar a una ruta de GitHub al publicar.

## 16. Build y distribución

- `scripts/build.ps1 -Version <ver>` (por defecto `dev`):
  1. comprobar `go`;
  2. `go generate ./assets` (sonidos, iconos y banner) y `go generate ./tools/gen-installer` (imágenes del asistente);
  3. `go generate ./winres` (icono y metadatos del .exe; si falla, sigue con el `.syso` ya commiteado);
  4. `go test ./...`; `go vet ./...`; comprobar `gofmt -l`;
  5. `go build -trimpath -ldflags "-s -w -X ...Version=<ver>" -o dist/tuick-tack.exe ./cmd/tuick-tack`;
  6. borrar cualquier `dist/tuick-tack-tray.exe` obsoleto;
  7. generar `dist/SHA256SUMS.txt` y mostrar la versión.
- `-Installer` (requiere una versión numérica y Inno Setup 6/`ISCC.exe`):
  8. compilar `installer/tuick-tack.iss` con `/DAppVersion=<ver sin v>` y `/DRepoRoot=<repo>`;
  9. verificar `dist/tuick-tack-setup.exe` e incluirlo en `SHA256SUMS.txt`.
- El setup es por usuario (`PrivilegesRequired=lowest`), instala en `{userdocs}\tuick-tack`, crea el acceso al menú Inicio y, como tarea opcional, el del escritorio; registra el desinstalador y conserva un autostart existente incluso en una actualización silenciosa (`InitializeSetup` + `CurStepChanged`). `[Run]` no lanza la app en modo silencioso: la reabre el helper.
- `tools/gen-installer` genera las dos imágenes de marca del asistente (`wizard-large.png` 164×314 con el icono, `12:30` en el 5×7 y el wordmark; `wizard-small.png` 55×55) desde `assets/icon.png` y la paleta Midnight. `go generate ./...` las regenera.
- `go-winres` genera un único `rsrc_windows_amd64.syso` desde `winres/winres.json` (icono multi-tamaño `assets/icon.ico`, manifiesto y versión); el `.syso` se commitea para que `go build` funcione sin regenerarlo. El script no reescribe `winres.json`: antes de una release hay que actualizar `FileVersion`/`ProductVersion` a la versión y regenerar el `.syso` (la versión del setup sale de `-Version`).
- `tools/gen-banner` genera `assets/banner.txt` (2x, 118 columnas), `assets/banner-compact.txt` (1x, 59) y `assets/banner.png` (celdas cuadradas) desde el mismo 5×7 de `bigfont` que el countdown. El PNG es transparente y el banner no se embebe en el ejecutable.
- Windows: `CGO_ENABLED=0`; no se necesita compilador C. Inno Setup solo se usa en build; el ejecutable no lo necesita para arrancar.
- Entrega Windows: carpeta `dist/` con `tuick-tack.exe`, `tuick-tack-setup.exe` (con `-Installer`) y `SHA256SUMS.txt`. Sin firma; SmartScreen puede avisar.
- **macOS (port congelado, manual):** `scripts/build-macos.sh build -version <ver> -arch <arm64|amd64>` compila con `CGO_ENABLED=1` (systray/cgo) y `-trimpath`; `package` combina las dos arquitecturas con `lipo`, monta `Tuick-Tack.app` desde `macos/Info.plist`, `macos/launcher` y `assets/icon.iconset`, firma ad-hoc binario y bundle, verifica con `codesign`, y escribe `dist/tuick-tack-macos-universal.zip`, `tuick-tack-macos-universal.zip.sha256` y `dist/SHA256SUMS.txt` (solo el zip). Un `CGO_ENABLED=0 GOOS=darwin go build ./...` debe compilar el fallback sin menu bar.
- La compilación real con cgo y el empaquetado se hacen en macOS o en un dispatch manual del workflow; el CI normal no compila macOS. `go generate ./...` incluye winres e Inno: no ejecutarlo en macOS ni en CI. `go generate ./assets` genera también los PNG de 32 px de la bandeja/menu bar.
- **Release Windows:** los dos `.exe` y `SHA256SUMS.txt` se publican con `gh release create`. El workflow `macos.yml` no se despacha en la release normal. Solo si se pide una release macOS, se despacha a mano con `tag=vX.Y.Z`, compila el commit del tag, verifica el zip y lo adjunta con su `.sha256` a una release existente; nunca crea la release. Firma de código/notarización fuera de alcance: la app es ad-hoc y Gatekeeper puede avisar en una descarga del navegador.
- `tuick-tack --version`; `tuick-tack --check-updates` (una comprobación de release y sale); `tuick-tack --install-startup` / `--uninstall-startup` escriben el autostart del propio ejecutable (registro en Windows; LaunchAgent en el port macOS congelado). `--apply-update` es interno del helper de Windows y no se documenta como CLI de usuario.
- README: requisitos, build, uso, teclas, presets, autostart, instalador/zip, actualizaciones, arquitectura, troubleshooting y licencia.

## 17. Tests y QA

### Automáticos

- `duration`: tabla de formatos válidos/inválidos, clamps.
- `config`: defaults, carga/guardado, corrupto → backup, migración v1→v7, v2→v7, v3→v7, v4→v7, v5→v7 y v6→v7 (incluido `readable_text=true` y `mini_timer=true`), validación de presets/fondo/sonido/volumen/tamaño y posición de ventana.
- `engine`: reloj falso; start/pause/resume/adjust/finish una vez/replace/restart; `finish_seq`.
- `protocol`: round-trip JSON, versión, errores, comandos de sonido y volumen, `set_background` válido/inválido y compatibilidad con un `config` viejo sin el campo, `set_readable_text` válido/sin `enabled` y `readable_text` puntero (presente/ausente), `toggle_mini_timer` válido, `set_window_size` y `hello.window` con y sin posición, `update`, `check_update`/`update_result`, `install_update`/`download_portable`, `update_progress`, `daemon_closing` con motivo, `get_stats`/`stats` y tolerancia a tipos desconocidos.
- `stats`: alta y recarga de JSONL, rechazo de registros inválidos, líneas corruptas saltadas, respaldo de un fichero ilegible, recorte al cap, agregados (día local, hora, día de la semana, totales por rango, top de etiquetas, recientes ordenados, récords) y rachas con huecos y mañana sin sesiones.
- `ipc`: pipe de test con nombre único, handshake con `update`, start/pause/stop, broadcast, múltiples clientes, reconexión y aviso final entregado antes de cerrar el pipe; en el port macOS congelado, socket Unix con ruta corta y limpieza.
- `update`: comparación semver y pre-releases, GET contra `httptest` (200/404/403/JSON roto/timeout), `User-Agent`, assets estables/digest, asset macOS congelado y checksum `.sha256`, `body`/notas con normalización de CRLF y recorte a 16 KiB en límite de rune, descarga TLS con progreso, SHA-256 correcto e incorrecto, checksums y rechazo de URLs no HTTPS.
- `install`: nombre visible, `InstallLocation`/`DisplayIcon`/`UninstallString`, comparación de rutas y detección del `.app` en `/Applications`/`~/Applications`.
- `updater`: copia del helper a un directorio temporal; en el port macOS congelado `Apply`/`Spawn` devuelven unsupported y `UpdateDir` apunta al caché; el lanzamiento real del setup se prueba a mano.
- `notify`: construcción del `Notification` (AUMID, título, cuerpo, URL de activación, duración) y validación; en el port macOS congelado, los argumentos de `osascript`; el COM real se prueba a mano.
- `browser`: solo `https`, rechazo de `http`, `file` y `javascript`.
- `canvas`/`bigfont`: dimensiones, clipping, ancho de runas, gradiente; `Render` de celdas transparentes (sin `FG`/`BG` no emite color, con reset al volver a una celda vacía) y `Dim` que solo toca colores presentes.
- `fx`: RNG con semilla, física, caducidad, sin NaN, límites; fondos deterministas, sin runas anchas, resize y contraste; la aurora conserva el hash del fondo anterior; estrellas solo en la franja superior; luciérnagas con centro y halo en la mitad inferior; Sunset con sol visible, aro brillante limitado y horizonte más claro; `none` deja el canvas intacto y transparente.
- `daemon`: nombres de mutex y `TUIRunning` (crear/consultar/liberar) con un pipe único; persistencia del tamaño y la posición de ventana, conservando la posición cuando el mensaje no la trae; persistencia de `set_background`, `set_readable_text` y `mini_timer`; parseo y migración del valor de autostart; catálogo de sonidos, escalado PCM y caché; caché de la release (incluidas las notas) y comprobación manual (up to date / available / failed) contra `httptest`; notificación inyectada solo para la bandeja; `Quit` idempotente y marcado como cierre pedido por el usuario; payload con capacidades setup/portable y notas; rechazos de instalación (`unavailable`, `busy`, `unsupported`, `timer_active`) y descarga+spawn con un `spawnUpdater` inyectado; historial con reloj inyectado: completed/stopped/replaced, pausas acumuladas, ajustes en el planificado, `onFinish` repetido no duplica y `get_stats` sale del store.
- `minitimer`: matriz de visibilidad (activado, estado, barra visible/horizontal, fullscreen, TUI con foco, flyout del shell); layout y DPI (3/4/5/6 px por celda, 128×30 a 100 %); texto MM:SS y H:MM:SS; progreso 0..1 con clamp y lleno al terminar; paleta por tema y estado; render del tiempo y la barra sin salirse de la rejilla.
- `tui`: `Update` con teclas; `View` contiene textos esperados en idle/running/paused/finished; `M` envía `toggle_mini_timer` y el Help muestra `[M] mini timer`; fondos ambientales con `b` (ciclo, toast, persistencia y fallback con daemon viejo) y layout sin runas anchas para cada fondo en 80×24/120×40; `none` con `BackgroundColor`/`ForegroundColor` nil en todos los fondos (la app nunca fija los colores de la terminal), canvas de fondo vacío, flash omitido y compacto transparente; bandas negras ajustadas al texto solo con `none` (cabecera, etiqueta, meta, input, atajos, sub-hint, status y compacto; el contador no), `n` persistido y fallback con daemon viejo, bandas desactivadas → sin bandas, y consulta OSC 11 (fondo claro/oscuro), paleta de escenario oscura en el camino adaptativo (contraste WCAG, sin cambios en fondo oscuro, desconocido u otro fondo) y ausencia de sombras de texto; colocación de ventana (aplicar una vez, ignorar el inicial, debounce, descartar guardados obsoletos, guardado al cerrar, consola compartida sin mover, maximizado sin guardar, repintado completo tras aplicar el tamaño, asentamiento de la posición que no guarda la cascada del host y compatibilidad con daemon viejo); `termwin` se prueba a mano por depender de la ventana real; modal de sonido; aviso de actualización (degradado animado con `animTime`, flecha, fallback corto, hover, click y modo compacto); `u` abre el diálogo si hay release o comprueba si no; diálogo en 48×22/80×24/120×40 con chips de tipo en una fila (copia en uso primero, estados `Installed`/`Not installed`/`In use`/`Not in use`/`Not available`; apilados a 44 columnas), selección con `Tab` (las flechas ya no cambian la copia), fila horizontal de acciones con `←`/`→`, aviso de la otra copia en una línea, aviso de countdown, progreso, resultado portable y fallo; panel de notas con `What's new`, scroll por `↑`/`↓`/`PgUp`/`PgDn`/`Home`/`End`/rueda, barra de scroll, hint dinámico (`[Tab] type`, `[↑/↓] notes`, `[g] github` según ancho y assets) y vuelta a 18 filas sin notas; `Enter`/`g`/`Esc`; aviso secundario `[u] update to vX`; celebración una vez por `finish_seq`; cierre por `daemon_closing` (quit/update) sin reconexión; estadísticas: `s` abre en idle (solo cuando no es sufijo), running, paused y finished; `25m`/`90s` siguen tecleándose; `m` mutea; `s`/`Esc` cierran; secciones con `←/→`/`Tab`/`1..4`, `t` cicla el rango, scroll y rueda, tabs clicables, refresco al reconectar y al parar con la pestaña abierta, cierre con aviso si el daemon es viejo, estado vacío, `s` ignorada en compacto y layout sin runas anchas en 48×22/80×24/120×40.
- `tui` (macOS, congelado): `singleUpdate` muestra una sola acción `download for macOS`, sin chips ni `Tab`, con badge `Portable · Tuick-Tack.app`, textos `macOS update`/`Unzip…` y progreso `Download · Verify · Done`; `New` activa el flag y `versionFromPath` entiende el sufijo del zip; el copy de autostart dice `Start at login` y el de menú bar, `menu bar`.
- `notes`: extracción de `What's new` (incluida la apóstrofe tipográfica) y fallback al body completo; tablas, enlaces, imágenes, énfasis, code fences, blockquotes y callouts; limpieza de ANSI, controles, emojis, CJK, bidi y entidades; wrapping con sangría colgante, palabras largas y anchos 20/36/56, todo de una celda.
- Comandos: `go test ./...`, `go vet ./...`, `gofmt -l .`; desde Windows, `CGO_ENABLED=0 GOOS=darwin go build ./...` y `GOOS=darwin go vet ./...` son una comprobación opcional del port congelado, no parte del CI normal.

### Manual — Windows

1. Abrir la TUI sin daemon → se arranca solo y conecta.
2. Iniciar 1 min; pausar/reanudar desde TUI y bandeja; ajustar ±1 min.
3. Cerrar la TUI con el timer corriendo → el tooltip cuenta; a 0 suena el sonido elegido; icono alerta.
4. Abrir la TUI después → pantalla de final con confeti; `r` repite; `Enter` cierra.
5. Lanzar un preset desde la bandeja con la TUI cerrada → sonido y alerta.
6. Añadir/editar/borrar preset → el submenú se actualiza; persistencia tras reiniciar.
7. Entradas `25`, `12:30`, `90s`, `1h30m`, inválida.
8. Temas; abrir `v`, recorrer los cuatro sonidos y probar 0/5/50/80/100 %; preview con `Space`; mute; “Start with Windows” on/off; “Quit”.
9. Migrar un config v1, v2 o v3 → aparece v4: v1 fija digital/80; v2 y v3 conservan sonido, volumen y tamaño; todos conservan presets, tema y mute y dejan la posición de ventana sin definir.
10. 48×22, 80×24 y 120×40; resize durante el confeti; con una TUI abierta, el clic en la bandeja la enfoca sin abrir otra; lanzar `tuick-tack.exe` a mano también la enfoca y sale.
11. El daemon (`--daemon --hidden`) no muestra consola; la TUI sí.
12. `dist/` contiene `tuick-tack.exe` y, con `-Version … -Installer`, `tuick-tack-setup.exe`; `go test`, build y ejecución desde `dist/`.
13. Instalar con `tuick-tack-setup.exe` en `Documentos\tuick-tack`: aparece el acceso al menú Inicio y la entrada de desinstalación; el asistente enseña el panel de marca y la versión. Desinstalar deja `%APPDATA%\tuick-tack` y `%LOCALAPPDATA%\tuick-tack` intactos.
14. Actualizar por encima de una instalación existente: se conserva el autostart y la config; la versión de Aplicaciones cambia. El setup silencioso no abre el asistente ni lanza la app por su cuenta.
15. Compilar con `-Version v0.3.0 -Installer`, salir antes del daemon de la bandeja y abrir: aparece `[Update available]` junto a la marca, el ítem en la bandeja y el click abre el diálogo. `--check-updates` imprime el resultado. Sin red, arranca normal y no avisa de errores.
16. Con una release real, pulsar `u`: la línea de estado muestra `Checking for updates…`, luego un toast interno (`vX available`, `Up to date…` o `Update check failed`) y el ítem de la bandeja refleja el mismo resultado. Repetirlo desde **Check for updates** de la bandeja: además debe salir un toast nativo de Windows; el de una release nueva abre la página al pulsar el cuerpo. La `u` debe verse atenuada debajo de los atajos, con una fila en blanco en medio, a 80×24.
17. Diálogo de actualización en la copia portable: el chip `Portable` sale primero como `In use` y seleccionado; `Tab` elige `Setup` (`Not installed`), aparece la línea ámbar `This is not the copy you use; settings carry over.` y la fila horizontal pasa a `install`. Si la release trae notas, el panel `What's new in vNueva` aparece con 8 líneas visibles a 80×24 y 16 a 120×40; `↑`/`↓`, `PgUp`/`PgDn`, `Home`/`End` y la rueda recorren las notas, la barra de scroll avanza y `Tab` sigue cambiando de copia sin perder la posición. Sin notas, el diálogo vuelve a 18 filas. `Enter` en `download` guarda `tuick-tack-vX.Y.Z.exe` en Descargas, muestra la ruta y `[o] open folder`; el timer sigue corriendo y la app no se cierra. En la copia instalada ocurre al revés: `Setup` primero como `Installed`, `Portable` como `Not in use`; `Enter` en `install` descarga el setup, pide confirmación si hay countdown, cierra la app, ejecuta el setup en silencio y reabre la TUI. Probar también un fallo de red y un checksum incorrecto: la versión vieja sigue.
18. Desactivar las notificaciones de Tuick-Tack o activar Focus Assist: la comprobación sigue y el estado de la bandeja cambia, aunque el toast no se vea.
19. Abrir el diálogo de actualización a 48×22: los chips de tipo se apilan, el panel de notas muestra menos líneas y el hint cae a la forma corta; comprobar que no hay runas anchas, solapes ni texto de control con una release que tenga emojis, enlaces y tablas. Repetir con una release sin sección `What's new`: debe caer al body completo, y con una sin `body`: el diálogo vuelve a 18 filas.
20. Con la TUI abierta (con y sin timer, y con un modal abierto), pulsar **Quit** en la bandeja: la TUI sale, el icono desaparece y no reaparece un daemon. `q` sigue cerrando solo la TUI.
21. Actualizar con un countdown corriendo: la TUI pide confirmación, el timer se pierde si se continúa y la app vuelve con el nuevo binario; la config se conserva.
22. Mover y redimensionar la ventana de la TUI, cerrar con `q` y reabrir desde la bandeja: vuelven la misma posición y el mismo tamaño, y `config.json` muestra `window_width`/`window_height` y `window_position`. Repetir cerrando con la X tras medio segundo. En Windows Terminal, comprobar que la ventana se mueve antes de redimensionar (el tamaño no se recorta) y que un terminal que ignore `CSI 8` conserva su tamaño. Maximizar, cerrar y reabrir: vuelve la última colocación normal, nunca la rejilla maximizada. Abrir la TUI desde una pestaña de Windows Terminal o con `go run`: no debe moverse la ventana completa ni guardarse su posición. Reiniciar el daemon con la TUI abierta: la ventana no salta.
23. Estadísticas: terminar un 5 min, parar otro antes de cero y reemplazar un tercero con un `start` nuevo. Abrir `s`: el overview suma las tres sesiones y el `% of finished` usa completed+stopped (el reemplazado no penaliza); con el rango de 7 días, las barras por día, la racha y el récord de sesión más larga cuadran. Cambiar a 30/90/all: la cabecera muestra el rango, el KPI cambia y el note de las barras dice la ventana real (`last N days`), no el rango. Revisar activity (heatmap de 26 semanas, hora de inicio y día, ambos all-time), presets (totales por etiqueta, sesiones y `% of finished`) y recent (`started`, `yesterday`/`yest`, y los tres outcomes). Reiniciar el daemon y reabrir: el historial sigue y `history.jsonl` tiene una línea por sesión. Repetir a 48×22, 80×24 y 120×40 en los tres temas: a 48 el KPI se parte en dos líneas, el eje por hora cae bajo sus barras, las etiquetas de 30 días no se desplazan y el activity puede pedir scroll; `t` solo cambia el rango en overview. Probar rueda, `PgUp`/`PgDn`, `Home`/`End` y un timer corriendo (abrir y cerrar la pestaña no lo toca).
24. Mini timer: comprobar que el checkbox **Mini timer** aparece solo en Windows, viene activado y persiste tras reiniciar el daemon. Iniciar 1 min: el widget aparece en el hueco izquierdo de la barra con `01:00` y la barra; pausar (ámbar), reanudar y esperar al final (`00:00`, barra llena). En idle desaparece. Enfocar la TUI lo oculta en menos de 500 ms; dejarla detrás lo vuelve a mostrar. Con la TUI en una pestaña inactiva de Windows Terminal debe seguir visible. Pulsar `M` en la TUI (idle, running, paused y finished): el checkbox de la bandeja cambia y el widget aparece/desaparece según el estado; `?` muestra la fila `[M] mini timer`.
25. Mini timer: clic izquierdo abre/enfoca la TUI y lo oculta; clic derecho abre Open/Pause/Resume/Stop/Hide mini timer; Hide desactiva el checkbox y persiste. Probar los tres temas y 100/125/150 % (sin suavizado ni solapes); auto-hide (solo con el cursor sobre la barra), fullscreen, menú Inicio/flyouts, reinicio de Explorer, multi-monitor y el posible solape con el botón de Widgets. En la barra superior horizontal debe seguir colocándose; en una barra vertical no debe dibujarse.

### Manual — macOS (port congelado; solo si se pide una release macOS)

No forma parte de la QA normal. El CI ya no compila macOS en cada push; la compilación se valida en un dispatch manual de `macos.yml`. Esta lista cubre lo que un runner headless no puede comprobar. Sin una Mac disponible, la release macOS se marca como experimental.

- [ ] Mover `Tuick-Tack.app` a `~/Applications`, abrirla con clic derecho > **Open** (o quitar la cuarentena) y confirmar que aparece el icono en el menu bar y no una ventana de terminal.
- [ ] `Open Tuick-Tack` en el menu bar abre la TUI en Terminal; una segunda TUI activa Terminal (o muestra `already open` si no puede) y no duplica el daemon.
- [ ] `Start 5 min`, `Pause`, `Resume` y `Stop` desde el menú; el título del menu bar cuenta atrás y al llegar a 0 suena el WAV y el icono cambia a alerta.
- [ ] Cambiar sonido, volumen y preview desde el menú y desde la TUI; `Sound` apagado silencia el fin.
- [ ] Cerrar la TUI y comprobar que el daemon, el countdown y el icono siguen; reabrir la TUI y ver el estado.
- [ ] `Start at login` crea `~/Library/LaunchAgents/com.juankyyy.tuick-tack.plist`, el icono aparece tras cerrar sesión y volver a entrar, y desactivarlo lo elimina. Mover la app y comprobar que el plist se reescribe al arrancar.
- [ ] **Check for updates** desde el menu bar muestra una notificación del sistema con el resultado; las notificaciones se pueden desactivar en Ajustes y el estado del menú sigue.
- [ ] Con una release real, `u` en la TUI muestra una sola acción `download for macOS`, sin chips ni `Tab`; descarga el zip en `~/Downloads`, lo verifica y Finder lo revela. Comprobar un fallo de red y un checksum incorrecto: la versión actual sigue.
- [ ] Sustituir el `.app` por el zip nuevo (cerrando antes el menu bar) y comprobar que config, historial, presets, tema y autostart se conservan.
- [ ] Config e historial en `~/Library/Application Support/tuick-tack`, caché en `~/Library/Caches/tuick-tack`.
- [ ] 48×22 (compacto), 80×24 y 120×40 en los tres temas; sin runas anchas, solapes ni recortes; la pestaña de estadísticas se desplaza a 48×22.
- [ ] Intel y Apple silicon: el zip es universal (`lipo -verify_arch arm64 x86_64`) y abre en ambas; `codesign --verify --deep --strict` pasa.

### Criterios de aceptación (v1)

- [ ] El timer es preciso y el finish ocurre una sola vez.
- [ ] Pausa/reanudación exactas; ajuste ±1 min.
- [ ] La bandeja sobrevive al cierre de la TUI; clic izquierdo abre la TUI o enfoca la abierta; clic derecho abre el menú.
- [ ] Una sola TUI: el tray y una segunda ejecución del exe enfocan la ventana existente sin duplicarla.
- [ ] Los presets (5/10/15 + personalizados) se inician desde TUI y bandeja; persisten.
- [ ] Sonido de fin de 1–5 s elegible entre cuatro, con volumen 0–100 y silencio; preview desde TUI y bandeja.
- [ ] La config v1 migra a v7 (digital/80, aurora, readable_text y mini_timer) y las versiones anteriores conservan sonido/volumen/tamaño/presets/tema, sin perder el ajuste de mini timer al venir de v6.
- [ ] El mini timer viene activado, se puede apagar desde la bandeja, con `M` en la TUI o con Hide, y persiste; el Help muestra `[M] mini timer`; aparece en running/paused/finished, se oculta en idle y con la TUI enfocada, y respeta auto-hide/fullscreen.
- [ ] El mini timer muestra solo tiempo y barra, con 128×30 px de base (143 px para horas de dos dígitos), los tres temas y bloques enteros a 100/125/150 %.
- [ ] El mini timer abre/enfoca la TUI con clic izquierdo y su menú contextual ofrece Open, Pause/Resume, Stop y Hide.
- [ ] Confeti a 0 en la TUI, una vez por final, ~6 s, sin artefactos al redimensionar.
- [ ] La UI es usable a 48×22 (modo tight), a 80×24, y se ve bien a 120×40.
- [ ] El tamaño y la posición de la ventana se recuerdan entre sesiones, no se guarda la colocación inicial ni se reaplica al reconectar, y una pestaña compartida no mueve el terminal entero.
- [ ] Un solo `.exe` portable; el daemon no abre consola con `--hidden`; autostart opcional funciona. El setup per-user instala, actualiza y desinstala sin admin.
- [ ] El aviso de release aparece en TUI y bandeja; el diálogo ofrece instalar con el setup o descargar el portable, verifica SHA-256, avisa si se pierde un countdown y conserva la config; la comprobación manual funciona desde la bandeja y con `u`.
- [ ] La comprobación desde la bandeja muestra un toast nativo con el resultado; el de una release nueva abre la página al pulsar.
- [ ] Las estadísticas registran cada sesión (completed/stopped/replaced) con sus tiempos, sobreviven al reinicio del daemon y se leen en 48×22, 80×24 y 120×40.
- [ ] Tests, vet y gofmt limpios.

### Criterios de aceptación (macOS congelado, solo para un build manual)

No son criterios de la release Windows. Solo se revisan si el usuario pide una release macOS.

- [ ] `CGO_ENABLED=0 GOOS=darwin go build ./...` y `GOOS=darwin go vet ./...` limpios desde Windows (comprobación opcional).
- [ ] El workflow manual compila con cgo, pasa tests y produce el zip universal verificado.
- [ ] El `.app` abre y muestra el menu bar; `Open Tuick-Tack` lanza la TUI; una segunda TUI no duplica el daemon.
- [ ] Sonido con `afplay`, notificación con `osascript` y autostart con LaunchAgent funcionan en una Mac.
- [ ] La actualización descarga el zip, verifica SHA-256, lo revela en Finder y no intenta instalar; `install_update` responde unsupported.
- [ ] Datos en `~/Library/Application Support` y `~/Library/Caches`; el build de macOS no usa el registro ni la config de Windows.
- [ ] Sin runas anchas en la TUI y layouts correctos a 48×22, 80×24 y 120×40.
- [ ] Gatekeeper/notarización documentados; la release se marca experimental si no hay QA en una Mac.

## 18. Hitos

| Hito | Entregable | Hecho cuando |
|---|---|---|
| **M0 · Setup + spike bandeja** | Go instalado, módulo, esqueleto, spike systray | Icono, tooltip, clic izq/dcho, menú y submenú funcionan en Windows |
| **M1 · Núcleo** | config, duration, engine, protocol + tests | `go test ./...` verde; reloj falso cubre pause/finish |
| **M2 · Daemon + IPC** | pipe, motor, sonido, instancia única, autostart | Dos clientes ven el mismo estado; sonido a 0; sin consola |
| **M3 · TUI funcional** | canvas, bigfont, layout, input, presets, modales | Se puede iniciar/pausar/ajustar/gestionar desde la TUI |
| **M4 · Estilo + celebración** | aurora, polvo, barra, confeti, finish | Confeti a 0 una vez, 30 fps, 80×24 y 120×40 |
| **M5 · Integración + release** | menú completo, iconos, abrir TUI, build, README | Criterios de aceptación completos; `dist/` listo |
| **M6 · macOS (congelado)** | capa de plataforma, menu bar, `.app` universal, update y CI manual | Build manual verificado en su día; el port queda congelado, sin release pendiente |
| **M7 · Mini timer (Windows)** | overlay de barra, config v7, toggle de bandeja, foco y auto-hide | Tests verdes y QA manual en Windows 10/11 a 100/125/150 % |

M6 no forma parte del plan activo.

## 19. Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| systray se comporta raro en Windows 11 | Spike M0; fallback `energye/systray` o tray Win32 mínimo |
| API/módulo de Bubble Tea v2 cambia | Versiones pinneadas; guía de upgrade v2; sin Bubbles |
| Fuente/or glyphs no disponibles | Windows Terminal + glifos box-drawing; fallback `* + •`; tests de ancho |
| Sin dispositivo de audio | Log y continuar; alerta visual/tooltip |
| .exe sin firmar / SmartScreen | Build local; firma fuera de alcance |
| Toast nativo poco fiable | Solo se usa para el resultado de la comprobación; el fin sigue con sonido + icono + TUI |
| Pérdida del timer al reiniciar el daemon | Documentado; futura persistencia |
| Pipe/instancia duplicada | Mutex + `first pipe instance`; test |
| `go-winres` falla | Build sin icono de .exe; icono de bandeja sigue |
| GitHub no responde, proxy o rate limit | Silencio; reintento a las 6 h; la app sigue normal |
| Descarga de actualización corrupta o manipulada | SHA-256 del asset/digest; se descarta y se conserva la versión actual |
| El setup falla o Windows bloquea el reemplazo | El helper registra en `update.log`, reabre la copia anterior y avisa con un toast; el portable no se toca |
| El usuario actualiza con un countdown activo | Aviso explícito y `force` en el protocolo; el timer se pierde solo tras confirmar |
| OneDrive sincroniza `Documentos` | La carpeta de instalación es configurable en el asistente; la config sigue en AppData |
| Notificaciones de Windows desactivadas o Focus Assist | El toast no se ve; el estado de la bandeja sigue mostrando el resultado |
| AUMID sin acceso directo | Verificado en Windows 10/11; si Windows lo rechazara, el estado de la bandeja sigue y el error queda en el log |
| Muchas animaciones consumen CPU | 10 Hz normal, 30 fps solo celebración; presupuesto < 8 ms/frame |
| Gatekeeper bloquea el `.app` descargado | Firma ad-hoc y README con clic derecho > Open o `xattr -dr com.apple.quarantine`; sin notarización asumida |
| cgo solo compila en macOS | Build manual en macOS; el CI normal no compila macOS |
| TCC al abrir Terminal desde el menu bar | `.command` en Application Support + `open`; no se usa AppleScript |
| Autostart con la app movida | El plist se reescribe al arrancar; se recomienda `~/Applications` |
| El update no puede reemplazar el `.app` solo | Descarga + verificación + Finder; el usuario cierra la app y sustituye |
| Intel desaparece en 2027 | Binario universal ahora; decidir después |
| La barra de tareas cambia de tamaño, DPI o se reinicia Explorer | Se vuelve a localizar `Shell_TrayWnd` y se recalcula la cápsula cada 500 ms; si no hay barra horizontal, se oculta |
| El overlay tapa el botón de Widgets/tiempo | Posición fija documentada; el usuario puede ocultar el mini timer desde la bandeja o el menú contextual |
| El overlay mantiene viva la barra auto-oculta | Con auto-hide solo se muestra con el cursor sobre la barra; al salir se oculta |
| fullscreen, presentación o flyouts del shell | `SHQueryUserNotificationState`, foco del shell y la ventana de la TUI se comprueban antes de mostrar |
| La ventana topmost roba el foco | `WS_EX_NOACTIVATE` + `SW_*NOACTIVATE`; solo el menú contextual usa `SetForegroundWindow` para cerrarse bien |

Las filas de macOS describen el port congelado y solo aplican si se retoma un build manual.

## 20. Fuera de alcance / futuro

> Las entradas de macOS están fuera de alcance mientras el port siga congelado; el código se
> conserva para un build manual puntual.

- Varios timers y cola.
- Persistencia/restauración del timer tras reinicio o crash.
- Mini timer en macOS o en una barra vertical/secundaria; el menu bar ya muestra el tiempo y el overlay solo cubre la barra horizontal principal.
- Toast nativo de Windows con AUMID para el fin del countdown (el aviso de actualizaciones ya usa uno).
- Hotkeys globales.
- MSIX, actualización automática sin confirmación, firma de código.
- Linux.
- Notarización y Developer ID en macOS, auto-reemplazo silencioso del `.app`, instalador/desinstalador tipo Inno y App Store/sandbox.
- Memoria de posición y tamaño de la ventana de la TUI en macOS.
- Sonidos personalizados por el usuario.
- Sincronización de presets entre máquinas.
- Entrada de estadísticas en el menú de la bandeja, exportación CSV y heatmap de 52 semanas (el historial guarda `paused_ms` para futuras métricas de pausas).

## 21. Anexos

### Ejemplo de config

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
    { "id": "p15", "label": "15 min", "seconds": 900 },
    { "id": "7f3c2a1e-…", "label": "Focus", "seconds": 1500 }
  ]
}
```

### Ejemplo de mensajes

```json
// cliente → daemon
{"type":"start","seconds":300,"label":"5 min","preset_id":"p5"}
{"type":"preset_add","label":"Focus","seconds":1500}
{"type":"set_sound_name","sound_name":"marimba"}
{"type":"set_volume","volume":75}
{"type":"set_window_size","width":140,"height":45,"position":{"x":93,"y":100}}
{"type":"check_update"}
{"type":"install_update","force":true}
{"type":"download_portable"}
{"type":"get_stats"}

// daemon → cliente
{"type":"hello","protocol":1,"app_version":"v0.2.0","config":{...},
 "window":{"width":140,"height":45,"position":{"x":93,"y":100}},
 "update":{"latest":"v0.3.0","url":"https://github.com/Juankyyy/tuick-tack/releases/tag/v0.3.0","setup":true,"portable":true,"notes":"## What's new in v0.3.0\n..."}}
{"type":"update","latest":"v0.3.0",
 "url":"https://github.com/Juankyyy/tuick-tack/releases/tag/v0.3.0","setup":true,"portable":true,"notes":"## What's new in v0.3.0\n..."}
{"type":"update_result","current":"v0.2.0","latest":"v0.3.0",
 "url":"https://github.com/Juankyyy/tuick-tack/releases/tag/v0.3.0","setup":true,"portable":true,"notes":"## What's new in v0.3.0\n..."}
{"type":"update_progress","kind":"setup","phase":"downloading","received":1048576,"total":5596614}
{"type":"update_progress","kind":"portable","phase":"done","path":"C:\\Users\\A\\Downloads\\tuick-tack-v0.3.0.exe"}
{"type":"update_progress","kind":"portable","phase":"done","path":"/Users/A/Downloads/tuick-tack-v0.4.0-macos-universal.zip"}
{"type":"stats","stats":{"daily":[{"date":"2026-11-18","ms":1500000,"n":2}],
 "hourly":[{"ms":0,"n":0}],"weekday":[{"ms":0,"n":0}],
 "labels":[{"label":"5 min","ms":1500000,"n":2,"completed":1,"stopped":1}],
 "records":{"streak_days":4,"best_streak_days":12,"longest_ms":3000000,
  "busiest_weekday":1,"busiest_hour":14},
 "recent":[{"started_ms":1762839120000,"planned_ms":300000,"elapsed_ms":300000,
  "label":"5 min","outcome":"completed"}],
 "totals":{"all":{"sessions":9},"d7":{"sessions":2},"d30":{"sessions":9},"d90":{"sessions":9}},
 "retained":9}}
{"type":"daemon_closing","reason":"update"}
{"type":"state","status":"running","duration_ms":300000,"remaining_ms":271500,
 "label":"5 min","preset_id":"p5","finish_seq":3,"end_at_unix_ms":1762839120000}
```

### Transporte local

- Windows: `\\.\pipe\tuick-tack-<usuario>-<sesión>` — usuario saneado a `[a-z0-9_]`; sesión obtenida con `ProcessIdToSessionId`.
- macOS (port congelado): `~/Library/Caches/tuick-tack/daemon-<uid>.sock`, modo `0600`; si la ruta supera 100 bytes el daemon falla con un error claro. El socket huérfano se borra si el connect falla.
