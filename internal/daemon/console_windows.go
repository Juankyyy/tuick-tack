package daemon

import "golang.org/x/sys/windows"

var procFreeConsole = windows.NewLazySystemDLL("kernel32.dll").NewProc("FreeConsole")

// DetachConsole releases the console inherited when the daemon is started from
// the Run key at sign-in, so the console window closes immediately and only the
// tray application remains. It is a no-op when the process has no console
// (CREATE_NO_WINDOW).
func DetachConsole() {
	if err := procFreeConsole.Find(); err != nil {
		return
	}
	_, _, _ = procFreeConsole.Call()
}
