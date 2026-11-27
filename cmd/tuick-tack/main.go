// Command tuick-tack is the single application executable. Without arguments
// it runs the terminal UI; with --daemon it runs the background daemon that
// owns the timer, presets, sound and tray icon. The TUI never owns the timer
// itself: it connects to the daemon over the local named pipe.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/daemon"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/tui"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/update"
	"tuick-tack/internal/updater"
)

func main() {
	// The update helper is a copy of this executable started by the daemon. It
	// runs before flag.Parse so its internal flags never reach the user-facing
	// command line.
	if len(os.Args) > 1 && os.Args[1] == "--apply-update" {
		os.Exit(updater.Apply(os.Args[2:]))
	}

	version := flag.Bool("version", false, "print the version and exit")
	checkUpdates := flag.Bool("check-updates", false, "check GitHub for a newer release and exit")
	daemonMode := flag.Bool("daemon", false, "run the background daemon in this process")
	hidden := flag.Bool("hidden", false, "hide the console when running the daemon (used by autostart)")
	launchHandled := flag.Bool(daemon.TUILaunchFlag, false, "internal: the launcher already chose the TUI window")
	installStartup := flag.Bool("install-startup", false, "start the tray daemon at sign-in")
	uninstallStartup := flag.Bool("uninstall-startup", false, "do not start the tray daemon at sign-in")
	flag.Parse()

	if *version {
		fmt.Printf("%s %s\n", buildinfo.Name, buildinfo.Version)
		return
	}
	if *checkUpdates {
		if err := checkForUpdates(); err != nil {
			fmt.Fprintln(os.Stderr, "check updates:", err)
			os.Exit(1)
		}
		return
	}
	if *daemonMode {
		if *hidden {
			daemon.DetachConsole()
		}
		os.Exit(daemon.Run())
	}
	if *installStartup {
		if err := daemon.InstallStartup(); err != nil {
			fmt.Fprintln(os.Stderr, "install startup:", err)
			os.Exit(1)
		}
		fmt.Println(copy.T(copy.StartupInstalled))
		return
	}
	if *uninstallStartup {
		if err := daemon.UninstallStartup(); err != nil {
			fmt.Fprintln(os.Stderr, "uninstall startup:", err)
			os.Exit(1)
		}
		fmt.Println(copy.T(copy.StartupRemoved))
		return
	}

	pipe, err := ipc.PipePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pipe:", err)
		os.Exit(1)
	}

	// A direct launch (shortcut, Explorer, installer) gets a console window
	// that Windows Terminal hosts with its "Defaults" profile, losing the
	// user's acrylic. Reopen it in a new Windows Terminal window with the
	// user's default profile when that is possible; the tray passes the marker
	// to skip this, and a shared shell tab is left untouched.
	if !*launchHandled {
		if handled, err := daemon.RelaunchTUI(pipe); handled {
			if err != nil {
				fmt.Println(copy.T(copy.AlreadyOpen))
			}
			return
		}
	}

	release, already, err := daemon.AcquireTUIInstance(pipe)
	if err != nil {
		fmt.Fprintln(os.Stderr, "instance:", err)
	} else {
		if already {
			if !daemon.FocusTUI() {
				fmt.Println(copy.T(copy.AlreadyOpen))
			}
			return
		}
		defer release()
	}

	model := tui.New(pipe)
	program := tea.NewProgram(model)
	final, err := program.Run()
	model.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuick-tack:", err)
		os.Exit(1)
	}
	if fm, ok := final.(*tui.Model); ok {
		switch {
		case fm.Updating():
			fmt.Println(copy.T(copy.ExitUpdating))
		case fm.QuitAll():
			fmt.Println(copy.T(copy.ExitQuitAll))
		case fm.DaemonClosing():
			fmt.Println(copy.T(copy.ExitDaemonClosing))
		case fm.HasActiveTimer():
			fmt.Println(copy.T(copy.ExitActive))
		}
	}
}

// checkForUpdates runs one release check for QA and scripts. It deliberately
// bypasses the daemon's dev-build guard.
func checkForUpdates() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	info, err := update.NewChecker(buildinfo.Version).Latest(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s\n", buildinfo.Name, buildinfo.Version)
	if update.IsNewer(buildinfo.Version, info.Version) {
		fmt.Printf("update available: %s\n%s\n", info.Version, info.URL)
	} else {
		fmt.Printf("up to date (latest %s)\n", info.Version)
	}
	return nil
}
