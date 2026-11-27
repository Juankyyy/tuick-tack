//go:build !windows && !darwin

package minitimer

import "log"

// Widget is a no-op on platforms without the Windows taskbar overlay.
type Widget struct{}

// New returns a no-op widget.
func New(Actions) *Widget { return &Widget{} }

// SetLogger is a no-op.
func (w *Widget) SetLogger(*log.Logger) {}

// Supported reports whether this build draws the taskbar widget.
func Supported() bool { return false }

// Run returns immediately.
func (w *Widget) Run() {}

// Close is a no-op.
func (w *Widget) Close() {}
