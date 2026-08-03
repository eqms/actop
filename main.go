// actop — a ctop-like terminal UI for Apple Container.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/config"
	"github.com/eqms/actop/ui"
)

var (
	version = "dev"
	build   = "unknown"
)

func main() {
	showVersion := flag.Bool("v", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("actop version %s, build %s\n", version, build)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v (using defaults)\n", err)
	}

	app := ui.NewApp(cli.New(), cfg, version)
	// alt-screen is requested by the model's View in Bubble Tea v2
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "actop: %v\n", err)
		os.Exit(1)
	}
}
