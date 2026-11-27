//go:build darwin

package minitimer

import "log"

// Widget is a no-op on macOS: the menu bar already shows the remaining time
// next to its icon, so there is no taskbar overlay to draw.
type Widget struct{}

// New returns the macOS no-op widget.
func New(Actions) *Widget { return &Widget{} }

// SetLogger is a no-op on macOS.
func (w *Widget) SetLogger(*log.Logger) {}

// Supported reports whether this build draws the taskbar widget.
func Supported() bool { return false }

// Run returns immediately; there is no native overlay to run.
func (w *Widget) Run() {}

// Close is a no-op on macOS.
func (w *Widget) Close() {}
