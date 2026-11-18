// Package winproc starts the console and hidden Windows processes that make up
// Tuick-Tack. It exists so the daemon, the tray launcher and the updater share
// the same CreateProcess rules.
package winproc

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	createNewConsole = 0x00000010
	createNoWindow   = 0x08000000
)

// StartHidden starts path without a console and releases the process. It is
// used for the background daemon and the update helper.
func StartHidden(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", path, err)
	}
	return cmd.Process.Release()
}

// StartConsole starts a console program in its own window. It uses Win32
// CreateProcess instead of os/exec on purpose: os/exec points unset stdio at
// NUL and sets STARTF_USESTDHANDLES, so the new console would stay blank while
// the child renders into NUL. Without that flag Windows attaches the child's
// stdin/stdout/stderr to the console created by CREATE_NEW_CONSOLE.
func StartConsole(path string, args ...string) error {
	exe, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var cmdLine *uint16
	if len(args) > 0 {
		line := windows.ComposeCommandLine(append([]string{path}, args...))
		cmdLine, err = windows.UTF16PtrFromString(line)
		if err != nil {
			return err
		}
	}
	si := &windows.StartupInfo{}
	si.Cb = uint32(unsafe.Sizeof(*si))
	pi := &windows.ProcessInformation{}
	if err := windows.CreateProcess(exe, cmdLine, nil, nil, false, createNewConsole, nil, nil, si, pi); err != nil {
		return fmt.Errorf("start %s: %w", path, err)
	}
	_ = windows.CloseHandle(pi.Thread)
	_ = windows.CloseHandle(pi.Process)
	return nil
}
