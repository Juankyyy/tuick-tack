# Tuick-Tack — SPEC

> **Estado:** implementado (v1) · **Fecha:** 2026-11-11
> **Plataforma:** Windows 10/11 x64 · **Licencia:** GPL-3.0 (repo)
> **Stack:** Go 1.26+ + Bubble Tea v2 + Lipgloss v2 + systray + go-winio
> **Repo:** `tuick-tack` (v1 implementado en `dev`; builds locales portables en `dist/`)

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
| Toast nativo | Fuera de alcance v1 (ver §16) |

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
- Estilo cuidado: fondo aurora, panel de cristal, dígitos grandes con degradado, barra de progreso viva.
- El countdown y el sonido sobreviven al cierre de la TUI.
- Configuración persistente, temas, autostart opcional, build reproducible de un solo `.exe`.

### No objetivos v1

- Varios timers simultáneos o cola de timers.
- Persistencia del timer en curso tras reiniciar el daemon.
- macOS/Linux (la app es Windows-only).
- Toast nativo de Windows obligatorio.
- Instalador/MSIX, auto-update, firma de código.
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

- **Ejecutable** = `tuick-tack.exe`, app de consola. Sin argumentos arranca la TUI; con `--daemon`, el daemon; con `--daemon --hidden`, el daemon sin consola (autostart).
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
6. `q` cierra solo la TUI; el daemon sigue. `Quit` en la bandeja cierra todo.
7. Al salir: parar sonido, cerrar el pipe, parar el engine y `systray.Quit()`.

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

- Transporte: named pipe de go-winio; mensajes JSON consecutivos con `json.Encoder`/`json.Decoder` sobre `net.Conn`.
- Handshake: cliente `hello {protocol: 1}` ↔ servidor `hello {protocol: 1, app_version, config}`.
- El servidor acepta N clientes; broadcast de estado a 10 Hz mientras corre (y en cada cambio), y config solo al cambiar.
- Errores: `{ "type": "error", "code": "...", "message": "..." }`.
- La TUI reintenta la conexión cada 1 s si se cae; muestra banner de desconexión y deshabilita comandos.

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
| `preview_sound` | — |
| `stop_sound` | — |
| `get_state` | — |
| `quit_daemon` | — |

Daemon → cliente:

| Mensaje | Contenido |
|---|---|
| `hello` | `protocol`, `app_version`, `config` |
| `state` | snapshot del §6 |
| `config` | `presets`, `theme`, `sound`, `sound_name`, `volume` |
| `error` | `code`, `message` |

Validación: duración 1..86400 s; etiqueta 1..32 caracteres; máximo 16 presets; tema válido; sonido válido (`chime`, `marimba`, `digital`, `deep`); volumen 0..100. Comando inválido → error, sin cambiar estado.

## 8. Configuración y presets

Rutas (por usuario, sin admin):

- Config: `%APPDATA%\tuick-tack\config.json` (`os.UserConfigDir`).
- Caché/logs: `%LOCALAPPDATA%\tuick-tack\` (`os.UserCacheDir`): `sounds\<sonido>-<volumen>.wav`, `logs\tuick-tack.log`.

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

- Semilla de defaults en el primer arranque; presets editables/borrables; “Restore defaults” en el gestor.
- Sonido por defecto `digital`; volumen por defecto 80, rango 0..100 y pasos de 5 en la UI.
- Migración v1 → v2 al cargar: conserva theme, sound y presets; fija `sound_name=digital` y `volume=80`. El daemon guarda la config migrada al arrancar.
- Escritura atómica (temporal en el mismo directorio + rename). Config corrupta: backup `config.bad-<timestamp>.json` y defaults.
- El daemon es el único escritor. La TUI manda comandos; el daemon guarda, actualiza el menú y hace broadcast.
- Etiqueta y duración visibles en el menú; el orden es el almacenado (defaults primero; los nuevos al final).
- Duraciones: parser en §9. Límite 1 s..24 h.

## 9. TUI

### Pantallas y estados

- **Idle:** dígitos fantasma `00:00`, input enfocado, chips de presets, footer de teclas. Mientras escribes una duración válida, los dígitos fantasma la previsualizan.
- **Running:** header con preset/estado, dígitos grandes, barra, “X remaining”, “ends at HH:MM”, footer de controles. Sin chips (foco).
- **Paused:** como running en ámbar, pill “PAUSED”, barra atenuada, “[Space] resume”.
- **Finished / celebración:** flash + confeti + “TIME'S UP!”; tras 6 s, pantalla tranquila con resumen y `r` repeat / `Enter` close.
- **Gestión de presets:** lista, añadir (`a`), editar (`e`), borrar (`d` con confirmación), restore defaults (`r`), start (`Enter`), close (`Esc`). `Tab` lleva el foco a la barra de acciones; `←/→` eligen acción y `Enter` la ejecuta. Las acciones que necesitan un preset se atenúan y se saltan si la lista está vacía.
- **Sonido:** modal `v` con on/off, lista Chime/Marimba/Digital/Deep, volumen 0–100 en pasos de 5, preview con `Space` y mute con `m`. Elegir sonido o cambiar volumen previsualiza si el sonido está activo.
- **Confirmación de cancelar:** overlay “Stop the countdown? Enter yes · Esc no”.
- **Ayuda:** overlay `?` con título “Help”, atajos globales y los atajos de sonido y presets (`[a] add`, `[e] edit`, `[d] delete`, `[r] defaults`).
- **Desconectado:** banner “Daemon disconnected — retrying…” y comandos deshabilitados.

### Layout

- Alt screen, cursor oculto, mouse habilitado para chips/botones.
- Mínimo 80×24. Modo compacto si ancho < 100 o alto < 30; modo amplio si no.
- Estructura vertical: header (1) · aire · hero dígitos (7) · etiqueta (1) · barra (1) · meta (1) · aire · [input + chips | controles] · footer (1).
- Todo centrado en una columna de máximo ~100 celdas.

### Teclas

Las pistas de la TUI usan el formato `[tecla] acción` (por ejemplo, `[Space] pause`); los botones conservan sus propios corchetes.

| Contexto | Acción |
|---|---|
| Idle | dígitos/`:`/`h`/`m`/`s` escriben; `Backspace` borra; `Enter` inicia; `←/→` seleccionan preset; `p` gestor; `t` tema; `v` sonido; `s` mute; `?` ayuda; `q` salir de la TUI |
| Running | `Space` pausa; `Esc` confirma cancelar; `+`/`-` ±1 min; `v` sonido; `?` ayuda; `q` salir |
| Paused | `Space` reanuda; `Esc` confirma cancelar; `+`/`-` ±1 min; `v` sonido; `q` salir |
| Finished | `Enter`/`Esc` cierran; `r` repiten; `v` sonido |
| Sonido | `↑/↓` sonido; `←/→` volumen ±5; `Space` preview; `m` mute; `Esc`/`Enter` cerrar |
| Gestor | `↑/↓` mover; `Tab` acciones; `←/→` elegir acción; `Enter` iniciar (lista) o ejecutar (acción); `a` añadir; `e` editar; `d` borrar; `r` defaults; `Esc` cerrar; en formulario `Tab` campo, `Enter` guardar, `Esc` cancelar |

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

- **Fondo aurora:** degradado vertical + dos halos radiales suaves; deriva lenta ligada al progreso.
- **Panel de cristal:** fondo un paso más claro, borde redondeado con degradado, 1 celda de margen interior.
- **Dígitos:** fuente de bloques 7×5, escala horizontal 1 en compacto y 2 en amplio, degradado por fila, sombra de 1 celda (abajo-derecha) y halo tenue.
- **Barra de progreso:** 1 fila, relleno con degradado accent→accent2, pista oscura, cabeza blanca brillante, marcas cada 25%.
- **Detalles vivos:** colon parpadeante, dot de estado pulsante, ~30 motas de polvo lentas, pulso ámbar en los últimos 10 s.
- **Sin emoji ni runas de doble ancho.** Todo el contenido usa glifos de una celda y box-drawing.
- **Truecolor**; Windows Terminal recomendado. `conhost` moderno también soporta 24-bit.

### Anti-patrones

- No llenar la pantalla de cajas y bordes.
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

- ~220 partículas, tope 260.
- Ráfagas: centro-abajo (70) a t=0, cañón izquierdo (55) a 0.2 s, derecho (55) a 0.4 s, lluvia superior (40) a 0.9 s.
- Física: gravedad de 180–300 px/s² escalada a celdas (~22–38 filas/s², una fila ≈ 8 px), rozamiento 0.985, deriva sinusoidal, giro, `dt` limitado a 50 ms.
- Glifos: `✦ ✧ ◆ ◇ • ▸` + alguna serpentina de 2 celdas; colores del tema + blanco.
- Estelas cortas en ~35% de las partículas durante el primer 40% de vida.
- Fade: mezcla del color hacia el fondo (no hay alpha en ANSI).
- Dibujo por capas: fondo → polvo → confeti → texto. La palabra y el resumen se dibujan encima para que el confeti pase por detrás.
- 30 fps con `tea.Every(33ms, …)`; parar el ticker cuando no queden partículas.
- RNG con semilla en tests; en producción semilla temporal.

## 12. Bandeja

`systray.Run(onReady, onExit)` ocupa el hilo principal; engine + IPC viven en goroutines. La API es global y segura desde goroutines.

### Menú

```
Tuick-Tack · 04:32 remaining
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
- **Start:** submenú reconstruido cuando cambian los presets; “Custom…” abre la TUI con el input enfocado.
- **Pause / Resume:** un solo ítem con título dinámico; habilitado según estado.
- **Stop:** habilitado en running/paused/finished; en finished actúa como dismiss.
- **Sound:** check; persiste en config y silencia el sonido de fin.
- **Sound settings:** submenú con los cuatro sonidos y check en el activo; Preview reproduce la selección. Elegir un sonido previsualiza si Sound está activo.
- **Volume:** submenú con título dinámico; -5%/+5% y atajos 0/25/50/75/100. Cada cambio previsualiza si Sound está activo.
- **Start with Windows:** check; lee/escribe `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, valor `TuickTack` apuntando a `"tuick-tack.exe" --daemon --hidden`. Sin admin. Al arrancar, el daemon migra un valor antiguo de `tuick-tack-tray.exe` para conservar el ajuste. En builds de `go run` el autostart queda deshabilitado: la ruta del ejecutable es temporal.
- **Quit:** para sonido, cierra pipe y `systray.Quit()`.

### Iconos

- 4 ICO embebidos (`idle`, `running`, `paused`, `finished`), 16/32/48 px; el `.ico` del ejecutable usa el de idle.
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

- `internal/tui/canvas`: rejilla de celdas `{ch rune; fg, bg color.RGBA; bold bool}` con `Set`, `Text`, `Box`, `Fill`, `Gradient`, `Blit`, `Center` y `Render() string` (ANSI SGR 24-bit, reset por fila, sin runas anchas).
- `internal/tui/bigfont`: fuente 7×5 para `0-9 : T I E M P O ¡ !`; dibuja con gradiente por fila y sombra.
- `internal/tui/theme`: paletas y utilidades de mezcla/lerp.
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
  daemon/                     # wiring, sound, autostart, logging
  tray/                       # systray menu, icons, tooltip
  tui/
    app.go                    # Bubble Tea model/update
    view.go                   # pantallas y layout
    copy/                     # todo el copy de la UI
    canvas/
    bigfont/
    theme/
    fx/                       # dust + confetti
assets/
  sounds/ chime.wav marimba.wav digital.wav deep.wav
  icon.ico
  icon.png                    # fuente 256 px para go-winres
  icons/ idle.ico running.ico paused.ico finished.ico
tools/
  gen-sounds/main.go
  gen-icons/main.go
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
  2. `go generate ./assets` (sonidos/iconos);
  3. `go generate ./winres` (icono y metadatos del .exe; si falla, sigue con el `.syso` ya commiteado);
  4. `go test ./...`; `go vet ./...`; comprobar `gofmt -l`;
  5. `go build -trimpath -ldflags "-s -w -X ...Version=<ver>" -o dist/tuick-tack.exe ./cmd/tuick-tack`;
  6. borrar cualquier `dist/tuick-tack-tray.exe` obsoleto;
  7. generar `dist/SHA256SUMS.txt` y mostrar la versión.
- `go-winres` genera un único `rsrc_windows_amd64.syso` desde `winres/winres.json` (icono `assets/icon.png`, manifiesto y versión); el `.syso` se commitea para que `go build` funcione sin regenerarlo.
- `CGO_ENABLED=0`; no se necesita compilador C.
- Entrega: carpeta `dist/` con `tuick-tack.exe` y `SHA256SUMS.txt`. Sin instalador. Firma de código fuera de alcance (SmartScreen puede avisar).
- `tuick-tack --version`; `tuick-tack --install-startup` / `--uninstall-startup` escriben el comando de autostart del propio ejecutable.
- README: requisitos, build, uso, teclas, presets, autostart, arquitectura, troubleshooting y licencia.

## 17. Tests y QA

### Automáticos

- `duration`: tabla de formatos válidos/inválidos, clamps.
- `config`: defaults, carga/guardado, corrupto → backup, migración v1→v2, validación de presets/sonido/volumen.
- `engine`: reloj falso; start/pause/resume/adjust/finish una vez/replace/restart; `finish_seq`.
- `protocol`: round-trip JSON, versión, errores, comandos de sonido y volumen.
- `ipc`: pipe de test con nombre único, handshake, start/pause/stop, broadcast, múltiples clientes, reconexión.
- `canvas`/`bigfont`: dimensiones, clipping, ancho de runas, gradiente.
- `fx`: RNG con semilla, física, caducidad, sin NaN, límites.
- `daemon`: nombres de mutex y `TUIRunning` (crear/consultar/liberar) con un pipe único; parseo y migración del valor de autostart; catálogo de sonidos, escalado PCM y caché.
- `tui`: `Update` con teclas; `View` contiene textos esperados en idle/running/paused/finished; modal de sonido; 80×24 y 120×40; celebración una vez por `finish_seq`.
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
9. Migrar un config v1 → aparece v2 con digital/80 y conserva presets, tema y mute.
10. 80×24 y 120×40; resize durante el confeti; con una TUI abierta, el clic en la bandeja la enfoca sin abrir otra; lanzar `tuick-tack.exe` a mano también la enfoca y sale.
11. El daemon (`--daemon --hidden`) no muestra consola; la TUI sí.
12. `dist/` contiene un solo `tuick-tack.exe`; `go test`, build y ejecución desde `dist/`.

### Criterios de aceptación (v1)

- [ ] El timer es preciso y el finish ocurre una sola vez.
- [ ] Pausa/reanudación exactas; ajuste ±1 min.
- [ ] La bandeja sobrevive al cierre de la TUI; clic izquierdo abre la TUI o enfoca la abierta; clic derecho abre el menú.
- [ ] Una sola TUI: el tray y una segunda ejecución del exe enfocan la ventana existente sin duplicarla.
- [ ] Los presets (5/10/15 + personalizados) se inician desde TUI y bandeja; persisten.
- [ ] Sonido de fin de 1–5 s elegible entre cuatro, con volumen 0–100 y silencio; preview desde TUI y bandeja.
- [ ] La config v1 migra a v2 (digital/80) sin perder presets, tema ni mute.
- [ ] Confeti a 0 en la TUI, una vez por final, ~6 s, sin artefactos al redimensionar.
- [ ] La UI es usable a 80×24 y se ve bien a 120×40.
- [ ] Un solo `.exe`; el daemon no abre consola con `--hidden`; autostart opcional funciona.
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
| Toast nativo poco fiable | No es requisito; sonido + icono + TUI |
| Pérdida del timer al reiniciar el daemon | Documentado; futura persistencia |
| Pipe/instancia duplicada | Mutex + `first pipe instance`; test |
| `go-winres` falla | Build sin icono de .exe; icono de bandeja sigue |
| Muchas animaciones consumen CPU | 10 Hz normal, 30 fps solo celebración; presupuesto < 8 ms/frame |

## 20. Fuera de alcance / futuro

- Varios timers y cola.
- Persistencia/restauración del timer tras reinicio o crash.
- Toast nativo de Windows con AUMID registrado.
- Hotkeys globales.
- Instalador, auto-update, firma.
- macOS/Linux.
- Sonidos personalizados por el usuario.
- Sincronización de presets entre máquinas.

## 21. Anexos

### Ejemplo de config

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

// daemon → cliente
{"type":"hello","protocol":1,"app_version":"0.1.1","config":{...}}
{"type":"state","status":"running","duration_ms":300000,"remaining_ms":271500,
 "label":"5 min","preset_id":"p5","finish_seq":3,"end_at_unix_ms":1762839120000}
```

### Nombre de pipe

`\\.\pipe\tuick-tack-<usuario>-<sesión>` — usuario saneado a `[a-z0-9_]`; sesión obtenida con `ProcessIdToSessionId`.
