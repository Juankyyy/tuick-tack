// Package assets embeds the generated runtime assets: the completion sound
// pack and the tray icons. The files are committed; regenerate them with
// `go generate ./...`.
package assets

import _ "embed"

//go:generate go run ../tools/gen-sounds
//go:generate go run ../tools/gen-icons

// ChimeWAV is the original 4-second bell arpeggio (44.1 kHz, 16-bit, mono).
//
//go:embed sounds/chime.wav
var ChimeWAV []byte

// MarimbaWAV is a warm wooden-mallet figure (44.1 kHz, 16-bit, mono).
//
//go:embed sounds/marimba.wav
var MarimbaWAV []byte

// DigitalWAV is a clean synth motif (44.1 kHz, 16-bit, mono).
//
//go:embed sounds/digital.wav
var DigitalWAV []byte

// DeepWAV is a low resonant gong (44.1 kHz, 16-bit, mono).
//
//go:embed sounds/deep.wav
var DeepWAV []byte

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
