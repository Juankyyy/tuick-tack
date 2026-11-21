package fx

import (
	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

// flat is the "none" background: it leaves the canvas untouched so the
// terminal's own background color or background image shows through. UI
// panels, buttons and modals still paint their own backgrounds.
type flat struct {
	w, h int
}

func (f *flat) Name() string { return config.BackgroundNone }

func (f *flat) Resize(w, h int) { f.w, f.h = w, h }

func (f *flat) Draw(_ *canvas.Canvas, _ theme.Theme, _ float64, _ State) {}
