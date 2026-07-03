package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/eqms/actop/redact"
)

// detailScreen shows one container's configuration, metrics and
// (redacted) environment.
type detailScreen struct {
	app   *App
	id    string
	vp    viewport.Model
	ready bool
}

func newDetailScreen(app *App, id string) *detailScreen {
	return &detailScreen{app: app, id: id}
}

func (d *detailScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "q", "esc":
			return nil, nil
		}
	case tea.WindowSizeMsg:
		d.ready = false
	}
	var cmd tea.Cmd
	d.vp, cmd = d.vp.Update(msg)
	return d, cmd
}

func (d *detailScreen) content() string {
	c, ok := d.app.store.Get(d.id)
	if !ok {
		return styleDim.Render("container disappeared")
	}
	var b strings.Builder
	line := func(k, v string) {
		fmt.Fprintf(&b, "%s %s\n", styleDim.Render(pad(k, 10)), v)
	}
	b.WriteString(styleTitle.Render(c.ID) + "\n\n")
	line("image", c.Image)
	line("state", stateStyle(c.State).Render(c.State))
	line("created", c.Created.Local().Format("2006-01-02 15:04:05"))
	if !c.Started.IsZero() {
		line("started", c.Started.Local().Format("2006-01-02 15:04:05"))
	}
	line("ip", c.IP)
	line("cpus", fmt.Sprintf("%d", c.CPUs))
	if c.Sample.MemLimit > 0 {
		line("memory", byteSize(c.Sample.MemUsage)+" / "+byteSize(c.Sample.MemLimit))
	}
	if c.Sample.PIDs > 0 {
		line("pids", fmt.Sprintf("%d", c.Sample.PIDs))
	}

	b.WriteString("\n" + styleColHead.Render("ENV") + "\n")
	if len(c.Env) == 0 {
		b.WriteString(styleDim.Render("(none)") + "\n")
	}
	for _, env := range c.Env {
		key, value, found := strings.Cut(env, "=")
		if !found {
			b.WriteString(env + "\n")
			continue
		}
		b.WriteString(key + "=" + redact.Env(key, value) + "\n")
	}
	return b.String()
}

func (d *detailScreen) view(width, height int) string {
	if !d.ready {
		d.vp = viewport.New(width, max(height-2, 1))
		d.ready = true
	}
	d.vp.SetContent(d.content())
	return d.vp.View() + "\n" + styleDim.Render("q/esc back · j/k scroll")
}
