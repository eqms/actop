package ui

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/config"
	"github.com/eqms/actop/update"
)

const (
	readTimeout   = 4 * time.Second
	actionTimeout = 15 * time.Second
)

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func fetchListCmd(c cli.Client, all bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
		defer cancel()
		items, err := c.List(ctx, all)
		return listResultMsg{items: items, err: err}
	}
}

func fetchStatsCmd(c cli.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
		defer cancel()
		entries, err := c.Stats(ctx)
		return statsResultMsg{entries: entries, ts: time.Now(), err: err}
	}
}

func fetchSystemStatusCmd(c cli.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
		defer cancel()
		st, err := c.SystemStatus(ctx)
		return systemStatusMsg{status: st, err: err}
	}
}

func fetchVolumesCmd(c cli.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
		defer cancel()
		vols, err := c.Volumes(ctx)
		return volumesResultMsg{volumes: vols, err: err}
	}
}

func fetchImagesCmd(c cli.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
		defer cancel()
		imgs, err := c.Images(ctx)
		return imagesResultMsg{images: imgs, err: err}
	}
}

func volumeDeleteCmd(c cli.Client, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionDoneMsg{action: "volume delete", id: name, err: c.VolumeDelete(ctx, name)}
	}
}

func imageDeleteCmd(c cli.Client, ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		return actionDoneMsg{action: "image delete", id: ref, err: c.ImageDelete(ctx, ref)}
	}
}

func actionCmd(c cli.Client, action, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		var err error
		switch action {
		case "start":
			err = c.Start(ctx, id)
		case "stop":
			err = c.Stop(ctx, id)
		case "kill":
			err = c.Kill(ctx, id)
		case "delete":
			err = c.Delete(ctx, id)
		}
		return actionDoneMsg{action: action, id: id, err: err}
	}
}

// openURLCmd opens the URL in the default browser via macOS open(1). The URL
// is assembled from CLI-reported ports and addresses, and open(1) launches an
// application or file just as readily as it opens a browser, so the scheme is
// verified before the value is handed over.
func openURLCmd(rawURL string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(rawURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return actionDoneMsg{action: "open", id: rawURL,
				err: fmt.Errorf("refusing to open %q: not an http(s) URL", rawURL)}
		}
		// #nosec G204 -- fixed binary, one argument past "--", scheme checked
		err = exec.Command("open", "--", u.String()).Start()
		return actionDoneMsg{action: "open", id: rawURL, err: err}
	}
}

// defaultShellProbe picks the best available shell inside the container.
// Fixed string, no user input is ever interpolated.
const defaultShellProbe = `if command -v fish >/dev/null 2>&1; then exec fish; ` +
	`elif command -v zsh >/dev/null 2>&1; then exec zsh; ` +
	`elif command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi`

// shellArgs resolves the configured shell (whitelisted) or the probe chain.
func shellArgs(cfg config.Config) []string {
	if cfg.Shell != "" && config.ShellAllowed(cfg.Shell) {
		return []string{cfg.Shell}
	}
	return []string{"sh", "-c", defaultShellProbe}
}

// execShellCmd suspends the TUI and attaches an interactive shell.
func execShellCmd(c cli.Client, cfg config.Config, id string) tea.Cmd {
	cmd := c.ExecCmd(id, shellArgs(cfg)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return execFinishedMsg{err: err}
	})
}

// logSession streams `container logs --follow` into tea messages.
type logSession struct {
	cancel func()
	ch     chan tea.Msg
}

func startLogSession(c cli.Client, id string) (*logSession, tea.Cmd, error) {
	cmd := c.LogsCmd(id, true)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan struct{})
	ch := make(chan tea.Msg, 256)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			select {
			// lines are sanitized where they enter the screen buffer
			// (logsScreen.update), so every rendering path is covered
			case ch <- logLineMsg{line: sc.Text()}:
			case <-done:
				return
			}
		}
		select {
		case ch <- logStreamEndMsg{err: sc.Err()}:
		case <-done:
		}
	}()
	s := &logSession{
		ch: ch,
		cancel: func() {
			close(done)
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		},
	}
	return s, waitForLogCmd(ch), nil
}

// waitForLogCmd delivers the next log message; the logs screen re-issues it
// after every received message (standard Bubble Tea channel consumption).
func waitForLogCmd(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// updateCheckTimeout bounds the whole release check. It runs off the update
// loop, so a slow answer delays nothing visible.
const updateCheckTimeout = 10 * time.Second

// checkUpdatesCmd asks the CLI for the installed container version and the
// checker for the latest releases. It returns nil when the user opted out,
// so no HTTP client is constructed and the CLI is never touched.
func checkUpdatesCmd(c cli.Client, cfg config.Config, version string, ck *update.Checker) tea.Cmd {
	if !cfg.UpdateCheckEnabled() {
		return nil
	}
	if ck == nil {
		ck = &update.Checker{}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		local := map[string]string{"actop": version}
		// A missing or broken container CLI only drops that one component;
		// Run skips entries it cannot parse.
		if v, err := c.Version(ctx); err == nil {
			local["container"] = v
		}
		return updatesMsg{result: ck.Run(ctx, local)}
	}
}
