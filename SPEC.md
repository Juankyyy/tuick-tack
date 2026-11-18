# Tuick-Tack — SPEC

> **Estado:** implementado (v1) · **Fecha:** 2026-11-12
> **Plataforma:** Windows 10/11 x64 · **Licencia:** GPL-3.0 (repo)
> **Stack:** Go 1.26+ + Bubble Tea v2 + Lipgloss v2 + systray + go-winio
> **Repo:** `tuick-tack` (v1 implementado en `dev`; builds locales `portable` y `setup` en `dist/`)

---

## 1. Resumen

Tuick-Tack es una app de escritorio para Windows con dos caras:

1. **TUI** en la terminal (Bubble Tea) para elegir presets, escribir minutos, ver el countdown, pausar, ajustar y gestionar presets.
2. **Icono en la bandeja** gestionado por un daemon en segundo plano que es dueño del temporizador, del sonido y del menú.

El countdown, el sonido y el icono siguen funcionando aunque se cierre la TUI. Al llegar a 0 suena el sonido elegido al volumen configurado y, si la TUI está abierta, se celebra con confeti a pantalla completa. Presets por defecto 5/10/15 min, presets personalizados con nombre y duración, y entrada libre de minutos.

## 2. Decisiones clave

| Área | Decisión |
|---|---|
| Lenguaje | Go 1.26+ (las versiones pinneadas de Bubble Tea v2 y go-winio lo requieren); `CGO_ENABLED=0` en Windows |
| TUI | `charm.land/bubbletea/v2` v2.1.1 + `charm.land/lipgloss/v2` v2.1.0 |
| Bandeja | `fyne.io/systray` v1.12.2 (Go puro en Windows; sin cgo) |
| IPC | `github.com/Microsoft/go-winio` v0.6.4, named pipe + JSON |
| Win32 | `golang.org/x/sys/windows` (`PlaySoundW`, mutex, registro) |
| Icono del .exe | `go-winres` generando un `.syso` |
| Arquitectura | Un ejecutable con dos modos (TUI y daemon), una sola cuenta atrás activa |
| Idioma | UI en inglés; código, comentarios y commits en inglés; documentación en español |
| Temas | `midnight` (por defecto), `ember`, `mono` |
| Aviso al terminar | Sonido elegible (1–5 s) con volumen 0–100 + icono de bandeja en alerta + confeti en la TUI |
| Estadísticas | Historial local `history.jsonl` (2000 registros), agregados en el daemon y pestaña `s` de la TUI con overview, actividad, presets y sesiones recientes |
| Toast nativo | Solo para el resultado de una comprobación manual de actualizaciones (`github.com/mtunjic/winrt-toast` v1.2.0); el aviso de fin sigue siendo sonido + TUI |

**Por qué un ejecutable y dos procesos:** un solo archivo evita que el usuario tenga que emparejar dos binarios. Por debajo siguen siendo dos procesos porque la TUI vive en una terminal: si fuese un solo proceso, cerrar la terminal mataría el icono, el countdown y el sonido. El daemon posee el estado; la TUI es una vista/controlador que se puede abrir y cerrar. Ambos modos viven en `tuick-tack.exe`: sin argumentos arranca la TUI y con `--daemon` el daemon.

**Por qué Go + Bubble Tea:** `systray` cubre exactamente el requisito (menú, submenús, tooltip, clic izquierdo/derecho, actualización dinámica) sin bucle de eventos propio; Bubble Tea/Lipgloss permite iterar el acabado visual rápido. Rust/ratatui se descartó por más ceremonia en la bandeja y el render.

## 3. Objetivos y no objetivos

### Objetivos

- Countdown preciso (reloj monotónico), con pausa/reanudación exactas.
- Presets 5/10/15 min editables + presets personalizados persistidos.
- Entrada libre de duraciones (`25`, `12:30`, `90s`, `1h30m`).
- Icono de bandeja con menú contextual para lanzar cualquier preset.
- Chime de 3–5 s al llegar a 0.
- Celebración con confeti en la TUI al llegar a 0.
- Estilo cuidado: fondo aurora a pantalla completa, cabecera de dos alturas, dígitos grandes con degradado y barra de progreso viva con checkpoints.
- El countdown y el sonido sobreviven al cierre de la TUI.
- Configuración persistente, temas, autostart opcional, build reproducible del `.exe` portable y de un setup por usuario.

### No objetivos v1

- Varios timers simultáneos o cola de timers.
- Persistencia del timer en curso tras reiniciar el daemon.
- macOS/Linux (la app es Windows-only).
- Toast nativo de Windows para el fin del countdown (las comprobaciones de actualizaciones sí usan uno, ver §12).
- MSIX, descarga o instalación automática sin confirmación, firma de código.
- Hotkeys globales, integración con calendario, sync en la nube.
- UI mouse-first (el ratón funciona para chips/botones, pero el teclado manda).

## 4. Arquitectura

```
        un solo ejecutable, dos procesos
┌───────────────────────────┐  named    ┌──────────────────────────────────────┐
│ tuick-tack.exe            │◄─pipe────►│ tuick-tack.exe --daemon              │
│ TUI (consola)             │  JSON     │ systray (hilo principal)             │
│ Bubble Tea v2             │  frames   │ TimerEngine · IPC server · config    │
│ canvas + bigfont + fx     │           │ PlaySoundW · volumen · iconos · tray │
└───────────────────────────┘           └──────────────────────────────────────┘
```

- **Ejecutable** = `tuick-tack.exe`, app de consola. Sin argumentos arranca la TUI; con `--daemon`, el daemon; con `--daemon --hidden`, el daemon sin consola (autostart). El mismo binario incluye el modo interno `--apply-update`, que usa el helper temporal de actualizaciones.
- **Instalador** = `tuick-tack-setup.exe`, generado con Inno Setup. Instala por usuario en `Documentos\tuick-tack`, sin admin, con acceso al menú Inicio y desinstalador. El portable sigue siendo `tuick-tack.exe`.
- **Daemon** = proceso `--daemon`. Es la única fuente de verdad: motor, config, sonido, bandeja. Al arrancar desde el registro libera su consola con `--hidden` y sigue como app de bandeja.
- **TUI** = proceso sin argumentos. Se conecta por el pipe; si el daemon no está, lanza una copia oculta de sí mismo con `--daemon` y reintenta.
- Una sola TUI por sesión; el tray reutiliza la ventana abierta. Los comandos afectan al único timer.
- El pipe es local, por usuario y sesión: `\\.\pipe\tuick-tack-<usuario>-<sesión>`. ACL para el usuario actual y acceso remoto denegado.
- Instancia única: mutex con nombre (`CreateMutexW`) + fallo al escuchar el pipe. Si ya hay daemon, el segundo proceso sale con código 0.
- Si el daemon se reinicia se pierde el timer en curso (documentado).

## 5. Ciclo de vida

1. El usuario abre `tuick-tack.exe`. Si ya hay una TUI (mutex `<pipe>-tui-mutex`), la nueva instancia enfoca la ventana existente y sale.
2. La TUI intenta conectar al pipe. Si falla:
   - Lanza una copia oculta de `tuick-tack.exe --daemon` detached con `CREATE_NO_WINDOW`.
   - Reintenta durante ~3 s (100 ms entre intentos).
3. El daemon arranca: logger, config, mutex, engine + servidor IPC en goroutines, `systray.Run` en el hilo principal.
4. La TUI recibe `hello` + config y renderiza.
5. Al llegar a 0, el daemon incrementa `finish_seq`, reproduce el sonido elegido y actualiza bandeja. La TUI detecta el nuevo `finish_seq` y lanza la celebración.
6. `q` cierra solo la TUI; el daemon sigue. `Quit` en la bandeja cierra todo: el daemon avisa `daemon_closing` a las TUIs conectadas y la TUI sale sin reintentar (si reintentase, `Ensure` relanzaría el daemon).
7. Al salir desde la bandeja: `systray.Quit()`, parar el loop, enviar `daemon_closing` a los clientes (se espera la escritura, con timeout), cerrar el pipe y parar el sonido. Un cierre de systray sin `Quit` (logoff) no envía el aviso.

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

- Transporte: named pipe de go-winio; mensajes JSON consecutivos con `json.Encoder`/`json.Decoder` sobre `net.Conn`. Los buffers del pipe son explícitos (64 KiB): con los tamaños por defecto una escritura puede esperar a que el cliente lea, y eso bloquearía los snapshots o el aviso final.
- Handshake: cliente `hello {protocol: 1}` ↔ servidor `hello {protocol: 1, app_version, config, update?, window?}`. `window` es aditivo y siempre lo envía un daemon de esta versión (`{width,height}` en celdas; ceros si no hay preferencia guardada). Su presencia indica que el daemon entiende `set_window_size`; un daemon viejo lo omite y la TUI no manda ese comando.
- El servidor acepta N clientes; broadcast de estado a 10 Hz mientras corre (y en cada cambio), y config solo al cambiar.
- Errores: `{ "type": "error", "code": "...", "message": "..." }`.
- La TUI reintenta la conexión cada 1 s si se cae; muestra banner de desconexión y deshabilita comandos.
- `daemon_closing` es la excepción: la TUI sale sin reintentar y sin lanzar un daemon nuevo, porque el cierre lo pidió el usuario desde la bandeja.
- Los mensajes de servidor desconocidos se decodifican como `IgnoredMessage` y se ignoran, para que un mensaje aditivo no rompa un cliente viejo. Por eso `update`, `update_progress` y `daemon_closing` no suben `Protocol`. El campo `notes` (cuerpo de la release, recortado a 16 KiB) es aditivo dentro de `update` y `update_result`: un daemon viejo no lo envía y una TUI vieja lo ignora. `install_update` y `download_portable` son comandos nuevos: un daemon viejo responde `invalid_command`, que la TUI muestra como actualización no disponible. `set_window_size` también es aditivo: la TUI solo lo envía si el `hello` trae `window`.

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
| `set_sound` | `enabled` |
| `set_sound_name` | `sound_name` |
| `set_volume` | `volume` |
| `set_window_size` | `width`, `height` (celdas; 1..1000) |
| `preview_sound` | — |
| `stop_sound` | — |
| `check_update` | — |
| `install_update` | `force?` (obligatorio si hay countdown activo) |
| `download_portable` | — |
| `get_state` | — |
| `get_stats` | — (pide el agregado del historial; solo responde al cliente que lo pide) |
| `quit_daemon` | — |

Daemon → cliente:

| Mensaje | Contenido |
|---|---|
| `hello` | `protocol`, `app_version`, `config`, `update?`, `window?` (`width`, `height`; siempre presente en daemons de esta versión) |
| `state` | snapshot del §6 |
| `config` | `presets`, `theme`, `sound`, `sound_name`, `volume` |
| `update` | `latest`, `url`, `setup?`, `portable?`, `notes?` |
| `update_result` | `current`, `latest?`, `url?`, `setup?`, `portable?`, `notes?`, `error?` |
| `update_progress` | `kind` (`setup`/`portable`), `phase`, `received?`, `total?`, `path?`, `error?` |
| `stats` | `daily` (180 días locales), `hourly` (24), `weekday` (7, lunes primero), `labels` (top 8), `records`, `recent` (20, más reciente primero), `totals` (`d7`/`d30`/`d90`/`all`), `retained` |
| `daemon_closing` | `reason?` (`quit`/`update`; la TUI sale sin reintentar) |
| `error` | `code`, `message` |

`notes` es el `body` de la release (Markdown UTF-8) recortado a 16 KiB. No es estado de la app: la TUI extrae su sección `What's new`, la limpia y la pinta en el diálogo; el daemon solo lo cachea y lo transporta.

`get_stats` y `stats` son aditivos: un daemon viejo responde `invalid_command` y la TUI cierra la pestaña con un aviso; un cliente viejo ignora `stats`. El daemon responde solo al cliente que lo pidió, como `update_result`. La TUI pide el agregado al abrir la pestaña, al reconectar y cuando un snapshot muestra un stop o un reemplazo con la pestaña abierta (un final la cierra para celebrar); el resto del tiempo no hay tráfico extra.

El tamaño de ventana no se difunde: es geometría del cliente, no estado compartido. El daemon lo guarda en config y lo devuelve en el siguiente `hello`; no hace broadcast ni refresca la bandeja.

Validación: duración 1..86400 s; etiqueta 1..32 caracteres; máximo 16 presets; tema válido; sonido válido (`chime`, `marimba`, `digital`, `deep`); volumen 0..100; tamaño de ventana 1..1000 por 1..1000 celdas. Comando inválido → error, sin cambiar estado.

## 8. Configuración y presets

Rutas (por usuario, sin admin):

- Config: `%APPDATA%\tuick-tack\config.json` (`os.UserConfigDir`).
- Historial de estadísticas: `%APPDATA%\tuick-tack\history.jsonl`, JSONL solo-añadir, escrito únicamente por el daemon.
- Caché/logs: `%LOCALAPPDATA%\tuick-tack\` (`os.UserCacheDir`): `sounds\<sonido>-<volumen>.wav`, `logs\tuick-tack.log`.

```json
{
  "version": 3,
  "theme": "midnight",
  "sound": true,
  "sound_name": "digital",
  "volume": 80,
  "window_width": 140,
  "window_height": 45,
  "presets": [
    { "id": "p5",  "label": "5 min",  "seconds": 300 },
    { "id": "p10", "label": "10 min", "seconds": 600 },
    { "id": "p15", "label": "15 min", "seconds": 900 }
  ]
}
```

- Semilla de defaults en el primer arranque; presets editables/borrables; “Restore defaults” en el gestor.
- Sonido por defecto `digital`; volumen por defecto 80, rango 0..100 y pasos de 5 en la UI.
- `window_width`/`window_height` son el tamaño de terminal preferido en celdas y se omiten mientras no haya preferencia (0). Solo los escribe el daemon cuando la TUI reporta un resize.
- Migración v1 → v3 y v2 → v3 al cargar: conserva theme, sound, presets, sonido y volumen; v1 fija `sound_name=digital` y `volume=80`; el tamaño de ventana queda sin definir. El daemon guarda la config migrada al arrancar.
- Escritura atómica (temporal en el mismo directorio + rename). Config corrupta: backup `config.bad-<timestamp>.json` y defaults.
- Historial de sesiones: una línea JSON por countdown terminado (`started_ms`, `planned_ms`, `elapsed_ms`, `paused_ms?`, `label`, `preset_id?`, `outcome`). `outcome` es `completed` (llega a 0, también ajustando), `stopped` (stop explícito) o `replaced` (un `start` lo sustituye). No se registra un daemon que muere con el timer vivo, igual que el timer no se restaura. Cap 2000 registros con recorte atómico e histéresis (256); las líneas inválidas se saltan y un fichero ilegible se respalda como `history.bad-<timestamp>.jsonl` y arranca vacío. **No vive en `config.json`**: es un log de datos, no ajustes versionados, y no se añade un segundo store de configuración ni una base de datos.
- El daemon es el único escritor. Para el historial guarda el inicio de pared al arrancar, acumula las pausas y suma los ajustes al planificado, de modo que el registro refleja el objetivo final y el tiempo real. El cálculo de días, horas y rachas usa la zona local y un reloj inyectable en tests.
- El daemon es el único escritor. La TUI manda comandos; el daemon guarda, actualiza el menú y hace broadcast del estado compartido. El tamaño de ventana solo se guarda y se devuelve en el siguiente `hello`.
- El registro se usa solo para autostart (`HKCU\...\Run`) y para el AUMID de las notificaciones (`HKCU\Software\Classes\AppUserModelId\Juankyyy.TuickTack`, valor `DisplayName`); nunca para estado de la app.
- Etiqueta y duración visibles en el menú; el orden es el almacenado (defaults primero; los nuevos al final).
- Duraciones: parser en §9. Límite 1 s..24 h.

## 9. TUI

### Pantallas y estados

- **Idle:** cabecera `READY` con `no countdown`, dígitos fantasma `00:00`, input enfocado, chips de presets y atajos de teclas bajo los presets. Mientras escribes una duración válida, los dígitos fantasma la previsualizan.
- **Running:** cabecera `RUNNING` con `ends at HH:MM`; dígitos grandes, barra, “X remaining”, botones y atajos de controles bajo los botones. Sin chips (foco).
- **Paused:** como running en ámbar, cabecera `PAUSED` con `on hold`, barra atenuada, meta con “[Space] resume”.
- **Finished / celebración:** flash + confeti + “TIME'S UP!”; la cabecera muestra `TIME'S UP!` y `at HH:MM`; tras 6 s, pantalla tranquila con resumen y `r` repeat / `Enter` close.
- **Gestión de presets:** lista, añadir (`a`), editar (`e`), borrar (`d` con confirmación), restore defaults (`r`), start (`Enter`), close (`Esc`). `Tab` lleva el foco a la barra de acciones; `←/→` eligen acción y `Enter` la ejecuta. Las acciones que necesitan un preset se atenúan y se saltan si la lista está vacía.
- **Sonido:** modal `v` con on/off, lista Chime/Marimba/Digital/Deep, volumen 0–100 en pasos de 5, preview con `Space` y mute con `m`. Elegir sonido o cambiar volumen previsualiza si el sonido está activo.
- **Confirmación de cancelar:** overlay “Stop the countdown? Enter yes · Esc no”.
- **Ayuda:** overlay `?` con título “Help”, atajos globales y los atajos de sonido y presets (`[a] add`, `[e] edit`, `[d] delete`, `[r] defaults`).
- **Estadísticas:** pestaña a pantalla completa con `s` (hoja sobre el escenario atenuado, como el diálogo de actualización). Cuatro secciones con `←/→`, `Tab` o `1..4`: **overview** (KPIs del rango y delta contra la ventana anterior, barras de foco por día con medio bloque y hoy en accent2, racha actual y mejor racha —días con al menos una sesión—, sesión más larga, día y hora más productivos); **activity** (heatmap de 26 semanas con celdas de fondo y 5 niveles, foco por hora de inicio y por día de la semana, frase de insight); **presets** (barras horizontales por etiqueta con runs y % completadas, más usado y media); **recent** (últimas 20 sesiones con `✓ completed` / `· stopped at 4m` / `· replaced at 8m` y scroll). `t` cicla 7/30/90/all en el overview; `↑/↓`, `PgUp`/`PgDn`, `Home`/`End` y la rueda desplazan; `Esc` o `s` cierran. Las gráficas usan 7 o 30 barras según el ancho y 6/8/10 filas de alto según el espacio (30 días con barras de una celda y etiquetas semanales). Bajo 48×22 no se dibuja y `s` se ignora. Abrir la pestaña no toca el timer; un final la cierra para enseñar la celebración, y un stop o un reemplazo desde la bandeja la dejan abierta y refrescan los datos.
- **Desconectado:** banner “Daemon disconnected — retrying…” y comandos deshabilitados.
- **Actualización disponible:** a la derecha de la marca y su versión, en la fila de la cabecera, `[Update available →]` en degradado pastel animado (arcoíris de 0,8 de vuelta a S 0,52 / L 0,68, negrita; la fase avanza ~0,1 vuelta/s con el tick de 10 Hz). Es clicable y abre el diálogo de actualización; al pasar el ratón por encima se pinta un fondo suave en toda la etiqueta. La flecha `→` es la pista estática de que abre algo. Si el estado no deja sitio, cae a `[Update →]`; si tampoco entra, no se dibuja. En `mono` usa un degradado de grises sin animación. El modo compacto añade una línea de texto sin click.
- **Buscar actualizaciones:** `u` en la pantalla principal pide una comprobación manual al daemon. Si ya hay una release conocida, `u` abre el diálogo en vez de volver a comprobar. Mientras comprueba, la línea de estado dice `Checking for updates…`; al terminar muestra un toast interno: `vX available`, `Up to date (latest vX)` o `Update check failed`. `u` funciona también en el modo compacto, pero allí no se dibuja el diálogo; la instalación requiere una ventana de al menos 48×22. La comprobación iniciada desde la bandeja muestra un toast nativo de Windows (ver §12).
- **Diálogo de actualización:** modal `Update available` con la transición `vActual → vNueva` en el tono apagado del texto secundario (estática en el diálogo de elección; en progreso un brillo suave la recorre), una etiqueta `Installed · <carpeta>` o `Portable · tuick-tack.exe` y los dos tipos en una fila compacta: la copia en uso primera y seleccionada (`› Setup · Installed` o `› Portable · In use`) y la otra al lado (`Portable · Not in use`, `Not installed` o `Not available` si falta el asset). A 44 columnas los dos chips se apilan. `↑`/`↓` y `Tab` cambian de copia cuando la release trae ambos assets; el hint anuncia `[Tab] type` cuando cabe. `←`/`→` mueven el foco entre la fila horizontal `install`/`download`, `github` y `later`; `Enter` ejecuta. Si el otro tipo falta, su fila muestra `Not available` y no se puede seleccionar. Al elegir la otra copia aparece una sola línea en ámbar, `This is not the copy you use; settings carry over.` (dos líneas por debajo de 50 columnas); no hay confirmación aparte. Si la release trae notas utilizables, el modal crece a 22 filas a 80×24 y a 30 filas a 120×40 y añade un panel `What's new in vNueva` con cabecera de sección en acento, viñetas, 8/16 líneas visibles y barra de scroll; `PgUp`/`PgDn`, `Home`/`End` y la rueda desplazan las notas sin cambiar la copia ni las acciones. Sin notas el diálogo vuelve a 18 filas y no reserva el panel. El hint completo es `[Enter] select · [Tab] type · [PgUp/Dn] notes · [Esc] later`; si no cabe, se quitan por este orden `[g] github`, las notas y `[Tab]` (`g` sigue abriendo GitHub y el botón `github` está a la vista). `g` abre GitHub, `Esc` cierra; `d`/`i` ya no existen y las flechas no se anuncian como atajo. Mientras se instala, el modal pasa a progreso (`Downloading…`, `Verifying…`, `Installing…`) con barra, porcentaje y pasos `Download · Verify · Install · Reopen`. Si hay un countdown corriendo o pausado, aparece `Countdown active` con el tiempo que se perderá y `[Enter] continue` / `[Esc] back`; el daemon exige el `force` de esa segunda confirmación. El resultado del portable muestra la ruta en Descargas, `[o] open folder` y `[Enter] close`; un fallo muestra `Update failed` con `[Enter] try again`, `[g] github` y `[Esc] close`.

### Layout

- Alt screen, cursor oculto, mouse en modo all-motion para chips/botones y el hover del aviso de actualización.
- **Tamaño de ventana recordado:** al conectar, si el daemon anuncia `window` con un tamaño guardado distinto del actual, la TUI pide al terminal `CSI 8;filas;columnas t` (lo soportan conhost y Windows Terminal moderno; uno que lo ignore se queda como está). Un resize real se envía al daemon con 300 ms de debounce y otra vez al cerrar con `q`/`Esc`; el tamaño inicial de arranque y el que produce aplicar la preferencia no se guardan. En reconexiones no se reaplica, para no pisar un resize del usuario. Se guardan celdas, no posición ni si estaba maximizada.
- Mínimo con estilos 48×22. El reloj usa escala 1 y pasa a escala 2 desde 100×30. La separación entre piezas es de una fila por debajo de 32 de alto y de dos filas desde 32 (la fila extra del aviso de actualización retrasa el cambio).
- Entre 22 y 23 filas se usa el modo *tight*: se omite la línea meta (`remaining` / `Type a duration or pick a preset`) y su hueco para que el bloque quepa sin solapes. Desde 24 filas vuelve la meta.
- Sin panel ni marco: la aurora ocupa toda la pantalla y el contenido vive en una columna centrada de máximo 100 celdas. En 80×24 quedan 4 celdas de margen a cada lado.
- Estructura vertical: cabecera de dos alturas (marca arriba; estado + sub-línea `ends at` / `on hold` / `at` / `no countdown` a la derecha) · aire · hero dígitos (7) · aire · etiqueta · aire · barra · aire · meta · aire · [input + chips | botones] · aire · atajos de teclas.
- El aviso de actualización vive en la fila de la marca, no añade filas: `● tuick·tack vX.Y.Z  [Update available]` con el estado a la derecha. La versión sigue a la marca en un tono apagado (el muted mezclado con el fondo al 85%) y cede su hueco si no deja sitio para la forma corta del aviso; el aviso se calcula contra el punto de estado y nunca lo pisa.
- Debajo de los atajos principales hay una fila en blanco y una segunda línea, más atenuada, con `[u] check for updates`, o `[u] update to vX` cuando ya hay una release conocida. En 80×24 la fila extra sale de juntar la etiqueta y la barra de progreso y de reducir el hueco entre la meta y los controles; desde 26 filas se mantiene el aire normal. En el modo tight (22–23 filas) y en compacto se omite para no quitar la línea de estado. La fila de estado sigue encima de los atajos.
- La cabecera se alinea a la medida y el bloque completo (dígitos → atajos) se centra verticalmente: los atajos quedan bajo los presets/botones, separados por el mismo aire (una fila por debajo de 30 de alto, dos desde 30), y el espacio sobrante queda bajo ellos. En 48×22 y 80×24 el bloque llena justo el alto disponible. La línea de estado (desconectado / aviso) se dibuja en la fila inmediatamente superior a los atajos.
- Por debajo de 48×22 (anchura < 48 o altura < 22) la TUI pasa a modo texto compacto: fondo plano, sin aurora, polvo, confeti, bigfont, chips ni botones. Muestra por prioridad el reloj, la etiqueta, el estado y los atajos esenciales, y recorta las líneas de menor prioridad hasta dejar solo el reloj en una fila. Los modales de gestión no se dibujan y sus teclas se ignoran; la confirmación de parada se muestra como texto. Al recuperar el mínimo se restaura la vista anterior.
- Los modales se dibujan sobre un scrim que atenúa el escenario y una hoja plana sin marco, con el mismo contenido y botones.

### Teclas

Las pistas de la TUI usan el formato `[tecla] acción` (por ejemplo, `[Space] pause`); los botones conservan sus propios corchetes.

| Contexto | Acción |
|---|---|
| Idle | dígitos/`:`/`h`/`m`/`s` escriben; `Backspace` borra; `Enter` inicia; `←/→` seleccionan preset; `p` gestor; `t` tema; `v` sonido; `m` mute; `s` estadísticas; `u` busca actualizaciones; `?` ayuda; `q` salir de la TUI |
| Running | `Space` pausa; `Esc` confirma cancelar; `+`/`-` ±1 min; `v` sonido; `m` mute; `s` estadísticas; `u` busca actualizaciones; `?` ayuda; `q` salir |
| Paused | `Space` reanuda; `Esc` confirma cancelar; `+`/`-` ±1 min; `v` sonido; `m` mute; `s` estadísticas; `u` busca actualizaciones; `?` ayuda |
| Finished | `Enter`/`Esc` cierran; `r` repiten; `v` sonido; `m` mute; `s` estadísticas; `u` busca actualizaciones |
| Estadísticas | `←/→`/`Tab` sección; `1..4` directo; `t` rango; `↑/↓` `PgUp/PgDn` `Home/End` rueda scroll; `Esc`/`s` cerrar |
| Sonido | `↑/↓` sonido; `←/→` volumen ±5; `Space` preview; `m` mute; `Esc`/`Enter` cerrar |
| Gestor | `↑/↓` mover; `Tab` acciones; `←/→` elegir acción; `Enter` iniciar (lista) o ejecutar (acción); `a` añadir; `e` editar; `d` borrar; `r` defaults; `Esc` cerrar; en formulario `Tab` campo, `Enter` guardar, `Esc` cancelar |
| Actualización | `↑/↓` o `Tab` tipo; `←/→` acción; `PgUp`/`PgDn` notas; `Home`/`End` principio/final; rueda notas; `Enter` ejecutar; `g` GitHub; `Esc` cerrar |

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
- El cierre de la TUI no toca el daemon. Al salir se restaura la terminal y se imprime “The countdown is still running in the tray.”
- Si el daemon se cierra desde la bandeja, envía `daemon_closing`, la TUI sale sin reintentar y sin relanzar el daemon, e imprime “The daemon quit; closing.” en vez del aviso del timer.

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

- **Fondo aurora:** degradado vertical + dos halos radiales suaves; deriva lenta ligada al progreso. Ocupa toda la pantalla, sin recorte por ningún panel.
- **Escenario abierto:** sin panel, sin marco y sin borde. El contenido flota directamente sobre la aurora en una columna centrada de hasta 100 celdas; no hay caja que simule una ventana.
- **Cabecera de dos alturas:** marca y dot a la izquierda; estado en mayúsculas y dot a la derecha; sub-línea derecha con `ends at HH:MM`, `on hold`, `at HH:MM` o `no countdown`. Sin píldora de estado.
- **Dígitos:** fuente de bloques 7×5, escala horizontal 1 en compacto y 2 en amplio, degradado por fila, sombra de 1 celda (abajo-derecha) y halo tenue.
- **Barra de progreso:** 1 fila, relleno con degradado accent→accent2, pista oscura, cabeza blanca brillante y checkpoints `┆` cada 25%.
- **Detalles vivos:** colon parpadeante, dot de estado pulsante, ~30 motas de polvo lentas, pulso ámbar en los últimos 10 s.
- **Sin emoji ni runas de doble ancho.** Todo el contenido usa glifos de una celda y box-drawing.
- **Truecolor**; Windows Terminal recomendado. `conhost` moderno también soporta 24-bit.

### Anti-patrones

- No llenar la pantalla de cajas y bordes.
- No simular una ventana: ni panel centrado, ni marco redondeado, ni píldora de estado.
- No cambiar el color del texto completo por estado; solo acentos, barra y cifras.
- No animar a 60 fps el idle: 10 Hz para polvo/estado; 30 fps solo en la celebración.

## 11. Celebración y confeti

### Secuencia

1. `finish_seq` cambia → flash de fondo (~120 ms) + arranca el sonido del daemon.
2. Ráfagas de confeti + “TIME'S UP!” en bloques grandes.
3. ~6 s de animación; el sonido dura entre 1,7 y 4 s según el pack.
4. Pantalla de celebración tranquila: “5 min completed”, hora, `r` repeat, `Enter` close.
5. `Enter`/`Esc` → idle y parar sonido; `r` → repetir la misma duración y volver al countdown.

### Confeti

- ~230 partículas base; la cifra escala por área hasta ~340 en 120×40, con tope 360.
- Ráfagas: popper central (60) a t=0, cañones izquierdo y derecho (35 cada uno) a 0.12/0.26 s, lluvia superior (70) a 0.55 s y segunda oleada (30) a 2.4 s. En una TUI que conecta con el temporizador ya terminado, una ráfaga corta de ~90.
- Física de papel: gravedad 11–16 filas/s² escalada, rozamiento dependiente de la orientación (de canto cae rápido; plano frena), caída terminal ~3–7 filas/s, deriva sinusoidal, bob vertical, giro 2.5–6 rad/s y `dt` limitado a 50 ms.
- Glifos de una celda: bloques `█ ▀ ▄ ▌ ▐ ▓ ▒ ░`, rombos `◆ ◇` y destellos `✦ ✧ ·`. El ángulo elige cara frontal, canto o dorso; el dorso se oscurece. Nada de emoji ni runas de doble ancho.
- Estelas de 2 celdas solo mientras la pieza va rápida (lanzamiento); el resto cae limpio. Fade al color real de la celda en el último 15% de vida y fundido global de 5.0 a 5.7 s.
- Al redimensionar, las posiciones se remapean proporcionalmente; las piezas que salen de pantalla se recortan.
- Dibujo por capas: fondo → polvo → confeti → texto. La palabra y el resumen se dibujan encima para que el confeti pase por detrás.
- 30 fps con `tea.Every(33ms, …)` durante la celebración; después vuelve el tick de 100 ms.
- RNG con semilla en tests; en producción semilla temporal.

## 12. Bandeja

`systray.Run(onReady, onExit)` ocupa el hilo principal; engine + IPC viven en goroutines. La API es global y segura desde goroutines.

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
Start with Windows
──────────────────────────
Quit
```

- **Clic izquierdo:** abre la TUI (`SetOnTapped`).
- **Clic derecho:** menú. No se define `SetOnSecondaryTapped` para que systray muestre el menú.
- **Status:** ítem deshabilitado con el tiempo restante; se actualiza a 1 Hz mientras corre.
- **Update available:** aparece solo cuando el daemon conoce una release más nueva; `Update available: vX.Y.Z`, habilitado y abre la release en el navegador. Se crea oculto justo después del status, así que al mostrarse conserva esa posición. No admite color: es un menú nativo.
- **Check for updates:** siempre visible. Pide una comprobación manual al daemon, muestra `Checking for updates…` y se deshabilita mientras corre, y luego queda como `Up to date (vX)` o `Update check failed` (se puede volver a pulsar). Cada comprobación desde aquí lanza además un toast nativo de Windows con el resultado; el de una release nueva abre la página al pulsar el cuerpo.
- **Start:** submenú reconstruido cuando cambian los presets; “Custom…” abre la TUI con el input enfocado.
- **Pause / Resume:** un solo ítem con título dinámico; habilitado según estado.
- **Stop:** habilitado en running/paused/finished; en finished actúa como dismiss.
- **Sound:** check; persiste en config y silencia el sonido de fin.
- **Sound settings:** submenú con los cuatro sonidos y check en el activo; Preview reproduce la selección. Elegir un sonido previsualiza si Sound está activo.
- **Volume:** submenú con título dinámico; -5%/+5% y atajos 0/25/50/75/100. Cada cambio previsualiza si Sound está activo.
- **Start with Windows:** check; lee/escribe `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, valor `TuickTack` apuntando a `"tuick-tack.exe" --daemon --hidden`. Sin admin. Al arrancar, el daemon migra un valor antiguo de `tuick-tack-tray.exe` para conservar el ajuste. En builds de `go run` el autostart queda deshabilitado: la ruta del ejecutable es temporal.
- **Quit:** cierra el loop y llama a `systray.Quit()`; avisa `daemon_closing` a las TUIs conectadas antes de cerrar el pipe, para que la TUI se cierre en vez de relanzar un daemon. Después para el sonido.

### Actualizaciones

- El daemon comprueba `https://api.github.com/repos/Juankyyy/tuick-tack/releases/latest` al arrancar (en segundo plano, sin bloquear tray/IPC) y después cada 24 h. Si falla (offline, proxy, 403 por rate limit), reintenta a las 6 h; solo queda en el log, sin toast.
- La comprobación manual se pide desde la bandeja (**Check for updates**) o desde la TUI con `u`. El daemon la ejecuta en una goroutine (las comprobaciones manuales y automáticas se serializan), actualiza la caché y responde `update_result` al cliente que la pidió; el estado de la bandeja pasa a `Checking…`, `Up to date (vX)` o `Update check failed`.
- La comprobación iniciada desde la bandeja muestra un toast nativo de Windows con el resultado: `Update available` (con la versión y la release como destino al pulsar), `You're up to date` (con la versión actual) o `Update check failed`. Se usa `Windows.UI.Notifications` vía `github.com/mtunjic/winrt-toast` v1.2.0 (COM puro, sin cgo ni PowerShell). Las comprobaciones automáticas y las iniciadas desde la TUI no muestran toast del sistema; la TUI ya tiene su propio aviso.
- El toast usa el AUMID `Juankyyy.TuickTack`, registrado una vez en `HKCU\Software\Classes\AppUserModelId` con `DisplayName=Tuick-Tack`. No requiere admin ni acceso directo en el menú Inicio para mostrarse; el `CustomActivator` solo haría falta para recibir callbacks, que no se usan (los toasts con release usan activación por protocolo). Focus Assist o las notificaciones desactivadas para la app pueden ocultarlo; el estado de la bandeja sigue siendo la fuente de verdad.
- Solo builds de release: `buildinfo.Version == "dev"` no comprueba automáticamente; la comprobación manual sí funciona en `dev`, pero `install_update` y `download_portable` se rechazan en builds `dev` porque el ejecutable es temporal. `tuick-tack --check-updates` hace una pasada y la imprime.
- Comparación semver propia (`internal/update`): acepta `v`, ignora metadata de build, pre-release antes que release; `dev` ordena antes que cualquier versión.
- La release expone los assets estables `tuick-tack-setup.exe`, `tuick-tack.exe` y `SHA256SUMS.txt`; `internal/update` los localiza por nombre y calcula el SHA-256 con el `digest` del asset o con el fichero de checksums. Si faltan, la acción correspondiente queda deshabilitada y solo se ofrece la página de la release.
- El resultado se cachea en memoria (con los assets y el `body` de la release), viaja en el `hello` y se emite como `update` cuando aparece. La TUI lo revalida contra su propia versión.
- **Notas de la release:** `internal/update` lee `body` del JSON, normaliza los saltos de línea y lo recorta a 16 KiB por límite de rune. La TUI muestra la sección entre el primer `##` que menciona `What's new` y el siguiente `##`; si no existe, cae al body completo. El renderizado (`internal/tui/notes`) quita Markdown, HTML, entidades, ANSI, emojis y cualquier runa que no sea de una celda, y envuelve a 56/34 columnas con sangría colgante.
- **Instalar update:** el daemon descarga el setup en `%LOCALAPPDATA%\tuick-tack\updates`, verifica el SHA-256 y exige `force` si hay un countdown corriendo o pausado. Después copia el ejecutable actual a un helper temporal, lo arranca oculto y cierra el daemon con `daemon_closing{reason:"update"}`. El helper espera a que el daemon y la TUI liberen sus mutex, ejecuta el setup con `/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CLOSEAPPLICATIONS`, busca la copia instalada (entrada de desinstalación de Inno) y reabre la TUI. Si el setup falla, relanza la copia anterior y muestra un toast.
- **Descargar portable:** el daemon descarga `tuick-tack.exe`, verifica el SHA-256 y lo guarda como `tuick-tack-vX.Y.Z.exe` en la carpeta Downloads del usuario (`FOLDERID_Downloads`). No reinicia ni cierra nada; la TUI muestra la ruta y ofrece abrirla en Explorer.
- Privacidad: una petición HTTPS anónima por comprobación, `User-Agent: tuick-tack/<versión>`, sin telemetría propia. Las descargas solo ocurren tras una confirmación explícita.
- El setup es por usuario, sin admin. Instala en `{userdocs}\tuick-tack`, crea el acceso al menú Inicio (y opcionalmente al escritorio) y registra el desinstalador; conserva la configuración y el autostart existente. La primera instalación manual debe hacerse con la app vieja cerrada (o el setup pedirá cerrarla); las siguientes actualizaciones desde la TUI ya coordinan el cierre. Sigue sin firma de código.

### Iconos

- Iconos de bandeja de píxeles duros, rejilla 7×7 con una celda de margen y sin antialiasing; estados play, agujas, pausa y 0 en el color del estado.
- 4 ICO embebidos (`idle`, `running`, `paused`, `finished`) con 16/20/24/32/40/48/56/64 px. `systray` carga el HICON de 2× (32/40/48/56/64) y el shell lo reduce a 16/20/24/28/32; la rejilla y el margen están pensados para esa reducción.
- El `.ico` del ejecutable (`assets/icon.ico`) es una loseta navy de 16×16 celdas con anillo mint y agujas blancas, con 16/24/32/40/48/64/96/128/256 px. `winres` lo embebe completo en el `.syso`; `assets/icon.png` es el máster de 256 px para previews.
- No se regenera el icono cada segundo: systray escribe cada icono nuevo en un archivo temporal. El tiempo va en el tooltip.
- Tooltip: `Tuick-Tack · ready`, `Tuick-Tack · 04:31 remaining (5 min)`, `Tuick-Tack · time's up!`.
- Actualización de título/estado/habilitados en cada snapshot (1 Hz) y reconstrucción del submenú de presets solo al cambiar config.

### Abrir la TUI

- `os.Executable()` (el propio `tuick-tack.exe`) → lanzar sin argumentos con `windows.CreateProcess` y `CREATE_NEW_CONSOLE`.
- No se usa `os/exec`: conecta stdin/stdout/stderr a `NUL` y activa `STARTF_USESTDHANDLES`, así que la consola nueva queda vacía y la TUI pinta en `NUL`. Con `CreateProcess` sin ese flag, Windows enlaza los handles del hijo con la consola recién creada.
- La TUI mantiene el mutex `<pipe>-tui-mutex`; una segunda instancia llama a `FocusTUI()` y sale.
- `OpenTUI` consulta `TUIRunning` antes de lanzar: si existe, enfoca la ventana y no crea ningún proceso.
- `FocusTUI` busca una ventana visible con título exacto `Tuick-Tack` y clase `ConsoleWindowClass` o `CASCADIA_HOSTING_WINDOW_CLASS`, la restaura si está minimizada, suelta el foreground lock con un toque de Alt y usa `SetForegroundWindow`; si falla, `SwitchToThisWindow`. El título exacto y la clase evitan coincidir con editores o con OpenChamber.
- Limitación: si la TUI está en una pestaña no activa de Windows Terminal, el título de la ventana no coincide; no se duplica, pero puede no enfocarse.
- No hay búsqueda de un segundo binario: TUI y daemon son el mismo archivo.

## 13. Sonido

- Pack de cuatro WAV originales, 44.1 kHz, 16-bit, mono, generado por `tools/gen-sounds` (sin samples con copyright):
  - `chime`: arpegio de campana de 4.0 s.
  - `marimba`: figura de madera de 2.8 s.
  - `digital`: motivo synth de 1.7 s (predeterminado).
  - `deep`: gong grave de 4.0 s.
- Los WAV viven en `assets/sounds/` y se embeben con `//go:embed`; `go generate ./assets` los regenera de forma determinista.
- Al preparar la reproducción, el daemon escala las muestras PCM por `volume/100` y escribe una variante en `%LOCALAPPDATA%\tuick-tack\sounds\<sonido>-<volumen>.wav` (escritura atómica, limpieza de variantes antiguas).
- Reproducción: `winmm.dll!PlaySoundW(path, 0, SND_FILENAME|SND_ASYNC|SND_NODEFAULT)`.
- Parar: `PlaySoundW(nil, 0, 0)` al dismiss, repetir, nuevo start, cerrar el modal o cambiar de sonido/volumen.
- Volumen 0 no reproduce nada. `sound=false` silencia el fin; el preview explícito funciona salvo a volumen 0.
- Elegir sonido o cambiar volumen desde la TUI o la bandeja previsualiza si `sound=true`; el preview no altera el volumen del sistema.
- Fallo de caché o audio → log y seguir; la bandeja y la TUI avisan igual.
- `SND_MEMORY` no se usa (no combina con `SND_ASYNC`); se usa archivo.

## 14. Render por celdas

- `internal/tui/canvas`: rejilla de celdas `{ch rune; fg, bg color.RGBA; bold bool}` con `Set`, `Text`, `Box`, `Fill`, `Gradient`, `Blit`, `Center`, `Dim` (scrim de modales) y `Render() string` (ANSI SGR 24-bit, reset por fila, sin runas anchas).
- `internal/tui/bigfont`: fuente 7×5 para `0-9 : T I E M P O ¡ !`; dibuja con gradiente por fila y sombra.
- `internal/tui/theme`: paletas y utilidades de mezcla/lerp.
- `internal/tui/notes`: extrae la sección `What's new` del body de la release, quita Markdown/HTML/ANSI/emoji y envuelve el texto a celdas; el resultado lo pinta el diálogo de actualización con scroll.
- `internal/tui/fx`: polvo ambiental y sistema de partículas del confeti.
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
  ipc/                        # server/client named pipe
  stats/                      # historial JSONL + agregados de la pestaña
  daemon/                     # wiring, sound, autostart, logging, update orchestration
  update/                     # release check, assets, download + SHA-256
  updater/                    # helper temporal que ejecuta el setup y reabre la TUI
  install/                    # detecta la copia instalada en el registro de Inno
  instance/                   # mutex de daemon/TUI compartidos
  winproc/                    # CreateProcess oculto y consola nueva
  explorer/                   # abrir Downloads con el fichero seleccionado
  browser/                    # ShellExecuteW para abrir la release
  notify/                     # toasts WinRT: AUMID, resultado de la comprobación
  tray/                       # systray menu, icons, tooltip
  tui/
    app.go                    # Bubble Tea model/update
    view.go                   # pantallas y layout
    stats.go                  # pestaña de estadísticas y sus gráficas
    copy/                     # todo el copy de la UI
    notes/                    # sección What's new -> líneas de una celda
    canvas/
    bigfont/
    theme/
    fx/                       # dust + confetti
assets/
  sounds/ chime.wav marimba.wav digital.wav deep.wav
  icon.ico                    # ICO multi-tamaño del ejecutable (winres)
  icon.png                    # máster 256 px para previews
  icons/ idle.ico running.ico paused.ico finished.ico
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
scripts/
  build.ps1
winres/
  winres.json                 # icono, manifiesto y versión del ejecutable
  generate.go                 # directiva go:generate del .syso
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
- `CGO_ENABLED=0`; no se necesita compilador C. Inno Setup solo se usa en build; el ejecutable no lo necesita para arrancar.
- Entrega: carpeta `dist/` con `tuick-tack.exe`, `tuick-tack-setup.exe` (con `-Installer`) y `SHA256SUMS.txt`. Firma de código fuera de alcance (SmartScreen puede avisar).
- `tuick-tack --version`; `tuick-tack --check-updates` (una comprobación de release y sale); `tuick-tack --install-startup` / `--uninstall-startup` escriben el comando de autostart del propio ejecutable. `--apply-update` es interno del helper y no se documenta como CLI de usuario.
- README: requisitos, build, uso, teclas, presets, autostart, instalador, actualizaciones, arquitectura, troubleshooting y licencia.

## 17. Tests y QA

### Automáticos

- `duration`: tabla de formatos válidos/inválidos, clamps.
- `config`: defaults, carga/guardado, corrupto → backup, migración v1→v3 y v2→v3, validación de presets/sonido/volumen/tamaño de ventana.
- `engine`: reloj falso; start/pause/resume/adjust/finish una vez/replace/restart; `finish_seq`.
- `protocol`: round-trip JSON, versión, errores, comandos de sonido y volumen, `set_window_size` y `hello.window`, `update`, `check_update`/`update_result`, `install_update`/`download_portable`, `update_progress`, `daemon_closing` con motivo, `get_stats`/`stats` y tolerancia a tipos desconocidos.
- `stats`: alta y recarga de JSONL, rechazo de registros inválidos, líneas corruptas saltadas, respaldo de un fichero ilegible, recorte al cap, agregados (día local, hora, día de la semana, totales por rango, top de etiquetas, recientes ordenados, récords) y rachas con huecos y mañana sin sesiones.
- `ipc`: pipe de test con nombre único, handshake con `update`, start/pause/stop, broadcast, múltiples clientes, reconexión y aviso final entregado antes de cerrar el pipe.
- `update`: comparación semver y pre-releases, GET contra `httptest` (200/404/403/JSON roto/timeout), `User-Agent`, assets estables/digest, `body`/notas con normalización de CRLF y recorte a 16 KiB en límite de rune, descarga TLS con progreso, SHA-256 correcto e incorrecto, checksums y rechazo de URLs no HTTPS.
- `install`: nombre visible, `InstallLocation`/`DisplayIcon`/`UninstallString`, comparación de rutas.
- `updater`: copia del helper a un directorio temporal; el lanzamiento real del setup se prueba a mano.
- `notify`: construcción del `Notification` (AUMID, título, cuerpo, URL de activación, duración) y validación; el COM real se prueba a mano.
- `browser`: solo `https`, rechazo de `http`, `file` y `javascript`.
- `canvas`/`bigfont`: dimensiones, clipping, ancho de runas, gradiente.
- `fx`: RNG con semilla, física, caducidad, sin NaN, límites.
- `daemon`: nombres de mutex y `TUIRunning` (crear/consultar/liberar) con un pipe único; persistencia del tamaño de ventana; parseo y migración del valor de autostart; catálogo de sonidos, escalado PCM y caché; caché de la release (incluidas las notas) y comprobación manual (up to date / available / failed) contra `httptest`; notificación inyectada solo para la bandeja; `Quit` idempotente y marcado como cierre pedido por el usuario; payload con capacidades setup/portable y notas; rechazos de instalación (`unavailable`, `busy`, `unsupported`, `timer_active`) y descarga+spawn con un `spawnUpdater` inyectado; historial con reloj inyectado: completed/stopped/replaced, pausas acumuladas, ajustes en el planificado, `onFinish` repetido no duplica y `get_stats` sale del store.
- `tui`: `Update` con teclas; `View` contiene textos esperados en idle/running/paused/finished; tamaño de ventana (aplicar una vez, ignorar el inicial, debounce, guardado al cerrar y compatibilidad con daemon viejo); modal de sonido; aviso de actualización (degradado animado con `animTime`, flecha, fallback corto, hover, click y modo compacto); `u` abre el diálogo si hay release o comprueba si no; diálogo en 48×22/80×24/120×40 con chips de tipo en una fila (copia en uso primero, estados `Installed`/`Not installed`/`In use`/`Not in use`/`Not available`; apilados a 44 columnas), selección con `↑`/`↓` y `Tab`, fila horizontal de acciones con `←`/`→`, aviso de la otra copia en una línea, aviso de countdown, progreso, resultado portable y fallo; panel de notas con `What's new`, scroll por `PgUp`/`PgDn`/`Home`/`End`/rueda, barra de scroll, hint dinámico (`[Tab] type`, `[PgUp/Dn] notes`, `[g] github` según ancho y assets) y vuelta a 18 filas sin notas; `Enter`/`g`/`Esc`; aviso secundario `[u] update to vX`; celebración una vez por `finish_seq`; cierre por `daemon_closing` (quit/update) sin reconexión; estadísticas: `s` abre en idle (solo cuando no es sufijo), running, paused y finished; `25m`/`90s` siguen tecleándose; `m` mutea; `s`/`Esc` cierran; secciones con `←/→`/`Tab`/`1..4`, `t` cicla el rango, scroll y rueda, tabs clicables, refresco al reconectar y al parar con la pestaña abierta, cierre con aviso si el daemon es viejo, estado vacío, `s` ignorada en compacto y layout sin runas anchas en 48×22/80×24/120×40.
- `notes`: extracción de `What's new` (incluida la apóstrofe tipográfica) y fallback al body completo; tablas, enlaces, imágenes, énfasis, code fences, blockquotes y callouts; limpieza de ANSI, controles, emojis, CJK, bidi y entidades; wrapping con sangría colgante, palabras largas y anchos 20/36/56, todo de una celda.
- Comandos: `go test ./...`, `go vet ./...`, `gofmt -l .`.

### Manual

1. Abrir la TUI sin daemon → se arranca solo y conecta.
2. Iniciar 1 min; pausar/reanudar desde TUI y bandeja; ajustar ±1 min.
3. Cerrar la TUI con el timer corriendo → el tooltip cuenta; a 0 suena el sonido elegido; icono alerta.
4. Abrir la TUI después → pantalla de final con confeti; `r` repite; `Enter` cierra.
5. Lanzar un preset desde la bandeja con la TUI cerrada → sonido y alerta.
6. Añadir/editar/borrar preset → el submenú se actualiza; persistencia tras reiniciar.
7. Entradas `25`, `12:30`, `90s`, `1h30m`, inválida.
8. Temas; abrir `v`, recorrer los cuatro sonidos y probar 0/5/50/80/100 %; preview con `Space`; mute; “Start with Windows” on/off; “Quit”.
9. Migrar un config v1 o v2 → aparece v3: v1 fija digital/80; v2 conserva sonido y volumen; ambos conservan presets, tema y mute y dejan el tamaño de ventana sin definir.
10. 48×22, 80×24 y 120×40; resize durante el confeti; con una TUI abierta, el clic en la bandeja la enfoca sin abrir otra; lanzar `tuick-tack.exe` a mano también la enfoca y sale.
11. El daemon (`--daemon --hidden`) no muestra consola; la TUI sí.
12. `dist/` contiene `tuick-tack.exe` y, con `-Version … -Installer`, `tuick-tack-setup.exe`; `go test`, build y ejecución desde `dist/`.
13. Instalar con `tuick-tack-setup.exe` en `Documentos\tuick-tack`: aparece el acceso al menú Inicio y la entrada de desinstalación; el asistente enseña el panel de marca y la versión. Desinstalar deja `%APPDATA%\tuick-tack` y `%LOCALAPPDATA%\tuick-tack` intactos.
14. Actualizar por encima de una instalación existente: se conserva el autostart y la config; la versión de Aplicaciones cambia. El setup silencioso no abre el asistente ni lanza la app por su cuenta.
15. Compilar con `-Version v0.3.0 -Installer`, salir antes del daemon de la bandeja y abrir: aparece `[Update available]` junto a la marca, el ítem en la bandeja y el click abre el diálogo. `--check-updates` imprime el resultado. Sin red, arranca normal y no avisa de errores.
16. Con una release real, pulsar `u`: la línea de estado muestra `Checking for updates…`, luego un toast interno (`vX available`, `Up to date…` o `Update check failed`) y el ítem de la bandeja refleja el mismo resultado. Repetirlo desde **Check for updates** de la bandeja: además debe salir un toast nativo de Windows; el de una release nueva abre la página al pulsar el cuerpo. La `u` debe verse atenuada debajo de los atajos, con una fila en blanco en medio, a 80×24.
17. Diálogo de actualización en la copia portable: el chip `Portable` sale primero como `In use` y seleccionado; `Tab` (o `↓`) elige `Setup` (`Not installed`), aparece la línea ámbar `This is not the copy you use; settings carry over.` y la fila horizontal pasa a `install`. Si la release trae notas, el panel `What's new in vNueva` aparece con 8 líneas visibles a 80×24 y 16 a 120×40; `PgUp`/`PgDn`, `Home`/`End` y la rueda recorren las notas, la barra de scroll avanza y `Tab` sigue cambiando de copia sin perder la posición. Sin notas, el diálogo vuelve a 18 filas. `Enter` en `download` guarda `tuick-tack-vX.Y.Z.exe` en Descargas, muestra la ruta y `[o] open folder`; el timer sigue corriendo y la app no se cierra. En la copia instalada ocurre al revés: `Setup` primero como `Installed`, `Portable` como `Not in use`; `Enter` en `install` descarga el setup, pide confirmación si hay countdown, cierra la app, ejecuta el setup en silencio y reabre la TUI. Probar también un fallo de red y un checksum incorrecto: la versión vieja sigue.
18. Desactivar las notificaciones de Tuick-Tack o activar Focus Assist: la comprobación sigue y el estado de la bandeja cambia, aunque el toast no se vea.
19. Abrir el diálogo de actualización a 48×22: los chips de tipo se apilan, el panel de notas muestra menos líneas y el hint cae a la forma corta; comprobar que no hay runas anchas, solapes ni texto de control con una release que tenga emojis, enlaces y tablas. Repetir con una release sin sección `What's new`: debe caer al body completo, y con una sin `body`: el diálogo vuelve a 18 filas.
20. Con la TUI abierta (con y sin timer, y con un modal abierto), pulsar **Quit** en la bandeja: la TUI sale, el icono desaparece y no reaparece un daemon. `q` sigue cerrando solo la TUI.
21. Actualizar con un countdown corriendo: la TUI pide confirmación, el timer se pierde si se continúa y la app vuelve con el nuevo binario; la config se conserva.
22. Cambiar el tamaño de la ventana de la TUI, cerrar con `q` y reabrir: vuelve el mismo tamaño y `config.json` muestra `window_width`/`window_height`. Repetir cerrando con la X tras medio segundo. En Windows Terminal, redimensionar y reabrir: la versión moderna aplica el tamaño; un terminal que ignore la secuencia se queda como está y no pisa la preferencia al abrir y cerrar sin redimensionar.
23. Estadísticas: terminar un 5 min, parar otro antes de cero y reemplazar un tercero con un `start` nuevo. Abrir `s`: el overview suma las tres sesiones y el % de completadas usa completed+stopped (el reemplazado no penaliza); las barras por día, la racha y el récord de sesión más larga cuadran. Revisar activity (heatmap de 26 semanas, hora y día), presets (totales por etiqueta, runs y %) y recent (`✓ completed`, `· stopped at`, `· replaced at`). Reiniciar el daemon y reabrir: el historial sigue y `history.jsonl` tiene una línea por sesión. Repetir a 48×22, 80×24 y 120×40 en los tres temas, con rueda y `PgUp`/`PgDn`, `t` cambiando el rango y un timer corriendo (abrir y cerrar la pestaña no lo toca).

### Criterios de aceptación (v1)

- [ ] El timer es preciso y el finish ocurre una sola vez.
- [ ] Pausa/reanudación exactas; ajuste ±1 min.
- [ ] La bandeja sobrevive al cierre de la TUI; clic izquierdo abre la TUI o enfoca la abierta; clic derecho abre el menú.
- [ ] Una sola TUI: el tray y una segunda ejecución del exe enfocan la ventana existente sin duplicarla.
- [ ] Los presets (5/10/15 + personalizados) se inician desde TUI y bandeja; persisten.
- [ ] Sonido de fin de 1–5 s elegible entre cuatro, con volumen 0–100 y silencio; preview desde TUI y bandeja.
- [ ] La config v1 migra a v3 (digital/80) y la v2 conserva sonido/volumen, sin perder presets, tema ni mute.
- [ ] Confeti a 0 en la TUI, una vez por final, ~6 s, sin artefactos al redimensionar.
- [ ] La UI es usable a 48×22 (modo tight), a 80×24, y se ve bien a 120×40.
- [ ] El tamaño de la ventana se recuerda entre sesiones y no se guarda el tamaño inicial ni se reaplica al reconectar.
- [ ] Un solo `.exe` portable; el daemon no abre consola con `--hidden`; autostart opcional funciona. El setup per-user instala, actualiza y desinstala sin admin.
- [ ] El aviso de release aparece en TUI y bandeja; el diálogo ofrece instalar con el setup o descargar el portable, verifica SHA-256, avisa si se pierde un countdown y conserva la config; la comprobación manual funciona desde la bandeja y con `u`.
- [ ] La comprobación desde la bandeja muestra un toast nativo con el resultado; el de una release nueva abre la página al pulsar.
- [ ] Las estadísticas registran cada sesión (completed/stopped/replaced) con sus tiempos, sobreviven al reinicio del daemon y se leen en 48×22, 80×24 y 120×40.
- [ ] Tests, vet y gofmt limpios.

## 18. Hitos

| Hito | Entregable | Hecho cuando |
|---|---|---|
| **M0 · Setup + spike bandeja** | Go instalado, módulo, esqueleto, spike systray | Icono, tooltip, clic izq/dcho, menú y submenú funcionan en Windows |
| **M1 · Núcleo** | config, duration, engine, protocol + tests | `go test ./...` verde; reloj falso cubre pause/finish |
| **M2 · Daemon + IPC** | pipe, motor, sonido, instancia única, autostart | Dos clientes ven el mismo estado; sonido a 0; sin consola |
| **M3 · TUI funcional** | canvas, bigfont, layout, input, presets, modales | Se puede iniciar/pausar/ajustar/gestionar desde la TUI |
| **M4 · Estilo + celebración** | aurora, polvo, barra, confeti, finish | Confeti a 0 una vez, 30 fps, 80×24 y 120×40 |
| **M5 · Integración + release** | menú completo, iconos, abrir TUI, build, README | Criterios de aceptación completos; `dist/` listo |

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

## 20. Fuera de alcance / futuro

- Varios timers y cola.
- Persistencia/restauración del timer tras reinicio o crash.
- Toast nativo de Windows con AUMID para el fin del countdown (el aviso de actualizaciones ya usa uno).
- Hotkeys globales.
- MSIX, actualización automática sin confirmación, firma de código.
- macOS/Linux.
- Sonidos personalizados por el usuario.
- Sincronización de presets entre máquinas.
- Entrada de estadísticas en el menú de la bandeja, exportación CSV y heatmap de 52 semanas (el historial guarda `paused_ms` para futuras métricas de pausas).

## 21. Anexos

### Ejemplo de config

```json
{
  "version": 3,
  "theme": "midnight",
  "sound": true,
  "sound_name": "digital",
  "volume": 80,
  "window_width": 140,
  "window_height": 45,
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
{"type":"set_window_size","width":140,"height":45}
{"type":"check_update"}
{"type":"install_update","force":true}
{"type":"download_portable"}
{"type":"get_stats"}

// daemon → cliente
{"type":"hello","protocol":1,"app_version":"v0.2.0","config":{...},
 "window":{"width":140,"height":45},
 "update":{"latest":"v0.3.0","url":"https://github.com/Juankyyy/tuick-tack/releases/tag/v0.3.0","setup":true,"portable":true,"notes":"## What's new in v0.3.0\n..."}}
{"type":"update","latest":"v0.3.0",
 "url":"https://github.com/Juankyyy/tuick-tack/releases/tag/v0.3.0","setup":true,"portable":true,"notes":"## What's new in v0.3.0\n..."}
{"type":"update_result","current":"v0.2.0","latest":"v0.3.0",
 "url":"https://github.com/Juankyyy/tuick-tack/releases/tag/v0.3.0","setup":true,"portable":true,"notes":"## What's new in v0.3.0\n..."}
{"type":"update_progress","kind":"setup","phase":"downloading","received":1048576,"total":5596614}
{"type":"update_progress","kind":"portable","phase":"done","path":"C:\\Users\\A\\Downloads\\tuick-tack-v0.3.0.exe"}
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

### Nombre de pipe

`\\.\pipe\tuick-tack-<usuario>-<sesión>` — usuario saneado a `[a-z0-9_]`; sesión obtenida con `ProcessIdToSessionId`.
