// actop — a ctop-like terminal UI for Apple Container.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

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
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "actop: %v\n", err)
		os.Exit(1)
	}
}
