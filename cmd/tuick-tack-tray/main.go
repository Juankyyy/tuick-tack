// Command tuick-tack-tray is the background daemon. It owns the timer, the
// configuration, the sound pack and the notification-area icon.
//
// In release builds this executable uses the windowsgui subsystem and does not
// open a console. In development it can be started from a terminal.
package main

import (
	"log"
	"os"

	"tuick-tack/internal/daemon"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("tuick-tack-tray: ")
	os.Exit(daemon.Run())
}
