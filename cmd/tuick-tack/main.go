// Command tuick-tack is the terminal UI. It connects to the daemon over the
// local named pipe and never owns the timer itself.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/daemon"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/tui"
	"tuick-tack/internal/tui/copy"
)

func main() {
	version := flag.Bool("version", false, "print the version and exit")
	daemonMode := flag.Bool("daemon", false, "run the background daemon in this process")
	installStartup := flag.Bool("install-startup", false, "start the tray daemon at sign-in")
	uninstallStartup := flag.Bool("uninstall-startup", false, "do not start the tray daemon at sign-in")
	flag.Parse()

	if *version {
		fmt.Printf("%s %s\n", buildinfo.Name, buildinfo.Version)
		return
	}
	if *daemonMode {
		os.Exit(daemon.Run())
	}
	if *installStartup {
		if err := daemon.InstallStartup(); err != nil {
			fmt.Fprintln(os.Stderr, "install startup:", err)
			os.Exit(1)
		}
		fmt.Println("Tuick-Tack will start with Windows.")
		return
	}
	if *uninstallStartup {
		if err := daemon.UninstallStartup(); err != nil {
			fmt.Fprintln(os.Stderr, "uninstall startup:", err)
			os.Exit(1)
		}
		fmt.Println("Tuick-Tack will no longer start with Windows.")
		return
	}

	pipe, err := ipc.PipePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pipe:", err)
		os.Exit(1)
	}
	model := tui.New(pipe)
	program := tea.NewProgram(model)
	final, err := program.Run()
	model.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuick-tack:", err)
		os.Exit(1)
	}
	if fm, ok := final.(*tui.Model); ok && fm.HasActiveTimer() {
		fmt.Println(copy.T(copy.ExitActive))
	}
}
