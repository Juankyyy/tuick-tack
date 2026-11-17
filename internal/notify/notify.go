// Package notify shows native Windows toast notifications for the result of a
// manual release check. The toast identity is a per-user AppUserModelID; no
// Start menu shortcut is needed to display it.
package notify

import (
	"fmt"
	"sync"

	"github.com/mtunjic/winrt-toast"

	"tuick-tack/internal/buildinfo"
)

// AppID is the per-user identity Windows shows above the toast. Registering it
// writes HKCU\Software\Classes\AppUserModelId\<AppID> with a display name; it is
// not app state and needs no administrator rights.
const AppID = "Juankyyy.TuickTack"

var (
	registerMu sync.Mutex
	registered bool
)

// Show displays one toast. launch, when set, is opened when the body is
// clicked. The title and message are already localized by the caller.
func Show(title, message, launch string) error {
	if err := register(); err != nil {
		return err
	}
	if err := toast.Push(notification(title, message, launch)); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	return nil
}

func notification(title, message, launch string) toast.Notification {
	return toast.Notification{
		AppID:               AppID,
		Title:               title,
		Message:             message,
		ActivationArguments: launch,
		Duration:            toast.DurationShort,
	}
}

// register is lazy so a build that never shows a notification does not touch
// the registry. A failure is retried on the next toast.
func register() error {
	registerMu.Lock()
	defer registerMu.Unlock()
	if registered {
		return nil
	}
	if err := toast.RegisterAppID(AppID, buildinfo.Name); err != nil {
		return fmt.Errorf("notify: register app id: %w", err)
	}
	registered = true
	return nil
}
