// actop — a ctop-like terminal UI for Apple Container.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/config"
	"github.com/eqms/actop/ui"
	"github.com/eqms/actop/update"
	"github.com/eqms/actop/upgrade"
)

var (
	version = "dev"
	build   = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "update-container" {
		os.Exit(runUpdateContainer(os.Args[2:]))
	}

	showVersion := flag.Bool("v", false, "print version and exit")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		_, _ = fmt.Fprintf(out, "Usage:\n  actop [-v]\n  actop update-container [-y]   update Apple Container, restarting running containers\n\n")
		flag.PrintDefaults()
	}
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

// runUpdateContainer wires upgrade.Runner to the real CLI, GitHub and the
// terminal, and returns the exit code: 0 when everything came back, 1 when
// the report needs attention or the run was refused.
func runUpdateContainer(args []string) int {
	fs := flag.NewFlagSet("update-container", flag.ContinueOnError)
	yes := fs.Bool("y", false, "do not ask before stopping running containers")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Ctrl+C reaches the whole foreground process group. Apple's updater
	// should stop on it; actop must not, or the service stays down. A
	// handler (unlike signal.Ignore) is reset on exec, so the child still
	// gets the default behaviour.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	go func() {
		for range sig {
			fmt.Fprintln(os.Stderr, "\ninterrupted -- bringing the container service back first")
		}
	}()

	stdin := bufio.NewReader(os.Stdin)
	r := &upgrade.Runner{
		Client: cli.New(),
		Latest: func(ctx context.Context) (string, bool) {
			return (&update.Checker{}).Latest(ctx, "container")
		},
		Script: update.ContainerScript,
		RunScript: func(ctx context.Context, path string) error {
			// #nosec G204 -- path is the update.ContainerScript constant
			cmd := exec.CommandContext(ctx, path)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			return cmd.Run()
		},
		Confirm: func(q string) bool {
			if *yes {
				return true
			}
			fmt.Printf("%s [y/N] ", q)
			answer, _ := stdin.ReadString('\n')
			a := strings.ToLower(strings.TrimSpace(answer))
			return a == "y" || a == "yes" || a == "j" || a == "ja"
		},
		Out: os.Stdout,
	}

	rep, err := r.Run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "actop update-container: %v\n", err)
		return 1
	}
	rep.Write(os.Stdout)
	if !rep.OK() {
		return 1
	}
	return 0
}
