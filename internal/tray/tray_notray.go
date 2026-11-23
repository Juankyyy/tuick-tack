//go:build darwin && !cgo

package tray

import "sync"

// Tray is the no-op menu-bar controller used when the binary is built without
// cgo. It keeps the daemon API identical, so the rest of the daemon compiles
// and runs; only the menu bar is missing. The real menu bar needs cgo and the
// systray bindings.
type Tray struct {
	actions Actions
	quit    chan struct{}
	once    sync.Once
}

// New returns a headless tray bound to actions.
func New(actions Actions) *Tray {
	return &Tray{actions: actions, quit: make(chan struct{})}
}

// Run blocks until Quit, mirroring the systray event loop.
func (t *Tray) Run() {
	<-t.quit
}

// Quit releases Run. It is safe from any goroutine and safe to call twice.
func (t *Tray) Quit() {
	t.once.Do(func() { close(t.quit) })
}

// Refresh does nothing without a menu bar.
func (t *Tray) Refresh() {}
