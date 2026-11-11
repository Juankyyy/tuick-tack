// Package assets embeds the generated runtime assets: the chime and the tray
// icons. The files are committed; regenerate them with `go generate ./...`.
package assets

import _ "embed"

//go:generate go run ../tools/gen-chime
//go:generate go run ../tools/gen-icons

// ChimeWAV is the 4-second completion chime (44.1 kHz, 16-bit, mono).
//
//go:embed chime.wav
var ChimeWAV []byte

// AppIcon is the icon used for the executable and the tray's default state.
//
//go:embed icon.ico
var AppIcon []byte

// IdleIcon is the tray icon used while no countdown is active.
//
//go:embed icons/idle.ico
var IdleIcon []byte

// RunningIcon is the tray icon used while a countdown runs.
//
//go:embed icons/running.ico
var RunningIcon []byte

// PausedIcon is the tray icon used while a countdown is paused.
//
//go:embed icons/paused.ico
var PausedIcon []byte

// FinishedIcon is the tray icon used when a countdown has reached zero.
//
//go:embed icons/finished.ico
var FinishedIcon []byte
