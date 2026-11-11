// Command tuick-tack is the terminal UI. It connects to the daemon over the
// local named pipe and never owns the timer itself.
package main

import (
	"flag"
	"fmt"
	"os"

	"tuick-tack/internal/buildinfo"
)

func main() {
	version := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *version {
		fmt.Printf("%s %s\n", buildinfo.Name, buildinfo.Version)
		return
	}
	fmt.Printf("%s %s\n\n", buildinfo.Name, buildinfo.Version)
	fmt.Println("The terminal UI is being built. Start the tray daemon with:")
	fmt.Println("  go run ./cmd/tuick-tack-tray")
	os.Exit(0)
}
