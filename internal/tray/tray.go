// Package tray owns the Windows notification-area icon and its menu.
//
// This is the M0 systray spike: it proves that the icon, tooltip, menu,
// submenu and click callbacks work. The daemon wiring in M2 replaces the
// placeholder actions while keeping this package as the only owner of
// systray calls.
package tray

import (
	"log"

	"fyne.io/systray"

	"tuick-tack/assets"
)

// Run starts the tray event loop. It blocks and must be called on the main
// goroutine; systray APIs are safe from other goroutines after OnReady.
func Run() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(assets.IdleIcon)
	systray.SetTooltip("Tuick-Tack · ready")
	systray.SetOnTapped(func() { log.Println("tray: left click") })

	status := systray.AddMenuItem("Tuick-Tack · ready", "Current countdown")
	status.Disable()
	systray.AddSeparator()
	open := systray.AddMenuItem("Open Tuick-Tack", "Open the terminal UI")
	start := systray.AddMenuItem("Start", "Start a preset")
	start5 := start.AddSubMenuItem("5 min", "Start 5 min")
	start10 := start.AddSubMenuItem("10 min", "Start 10 min")
	start15 := start.AddSubMenuItem("15 min", "Start 15 min")
	startCustom := start.AddSubMenuItem("Custom…", "Type a duration in the TUI")
	pause := systray.AddMenuItem("Pause", "Pause the countdown")
	stop := systray.AddMenuItem("Stop", "Stop the countdown")
	sound := systray.AddMenuItemCheckbox("Sound", "Play the chime at zero", true)
	startup := systray.AddMenuItemCheckbox("Start with Windows", "Launch the daemon at sign-in", false)
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "Quit Tuick-Tack")

	go func() {
		for {
			select {
			case <-open.ClickedCh:
				log.Println("tray: open")
			case <-start5.ClickedCh:
				log.Println("tray: start 5 min")
			case <-start10.ClickedCh:
				log.Println("tray: start 10 min")
			case <-start15.ClickedCh:
				log.Println("tray: start 15 min")
			case <-startCustom.ClickedCh:
				log.Println("tray: custom")
			case <-pause.ClickedCh:
				log.Println("tray: pause/resume")
			case <-stop.ClickedCh:
				log.Println("tray: stop")
			case <-sound.ClickedCh:
				if sound.Checked() {
					sound.Uncheck()
				} else {
					sound.Check()
				}
			case <-startup.ClickedCh:
				if startup.Checked() {
					startup.Uncheck()
				} else {
					startup.Check()
				}
			case <-quit.ClickedCh:
				log.Println("tray: quit")
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	log.Println("tray: exit")
}
