package ui

import (
	"fmt"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// systemScreen is the system tab: API server info and resource counters.
type systemScreen struct {
	app *App
}

func newSystemScreen(app *App) *systemScreen {
	return &systemScreen{app: app}
}

func (s *systemScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "q", "ctrl+c":
		return s, tea.Quit
	case "h", "?":
		return s, s.app.push(newHelpScreen())
	}
	return s, nil
}

func (s *systemScreen) view(width, height int) string {
	var b strings.Builder
	line := func(k, v string) {
		fmt.Fprintf(&b, "  %s %s\n", styleDim.Render(pad(k, 12)), v)
	}

	b.WriteString("\n  " + styleColHead.Render("API SERVER") + "\n")
	if st := s.app.sysInfo; st != nil {
		status := safe(st.Status)
		if st.Status == "running" {
			status = styleRunning.Render(status)
		} else {
			status = styleStopped.Render(status)
		}
		line("status", status)
		line("version", safe(st.APIServerVersion))
		line("app root", safe(st.AppRoot))
	} else if !s.app.connOK {
		line("status", styleStopped.Render("not reachable"))
	} else {
		line("status", styleDim.Render("loading…"))
	}

	running := 0
	for _, c := range s.app.store.Snapshot("", "name", false) {
		if c.Running() {
			running++
		}
	}
	b.WriteString("\n  " + styleColHead.Render("RESOURCES") + "\n")
	line("containers", fmt.Sprintf("%d (%d running)", s.app.store.Len(), running))
	line("volumes", fmt.Sprintf("%d", len(s.app.volumes)))
	line("images", fmt.Sprintf("%d", len(s.app.images)))

	var volBytes int64
	for _, v := range s.app.volumes {
		volBytes += v.Configuration.SizeInBytes
	}
	if volBytes > 0 {
		line("volume size", byteSize(volBytes)+" (allocated)")
	}

	b.WriteString("\n  " + styleColHead.Render("HOST") + "\n")
	line("cpus", fmt.Sprintf("%d", runtime.NumCPU()))
	line("actop", fmt.Sprintf("v%s (poll %s)", s.app.version, s.app.cfg.PollInterval()))
	return b.String()
}
