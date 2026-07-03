package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/eqms/actop/model"
)

// gridScreen is the base screen: the compact container table.
type gridScreen struct {
	app       *App
	cursor    int
	filter    textinput.Model
	filtering bool
}

func newGridScreen(app *App) *gridScreen {
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.CharLimit = 64
	return &gridScreen{app: app, filter: ti}
}

func (g *gridScreen) rows() []*model.Container {
	return g.app.store.Snapshot(g.filter.Value(), g.app.sortField, g.app.sortReversed)
}

func (g *gridScreen) selected() *model.Container {
	rows := g.rows()
	if len(rows) == 0 {
		return nil
	}
	if g.cursor >= len(rows) {
		g.cursor = len(rows) - 1
	}
	return rows[g.cursor]
}

func (g *gridScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return g, nil
	}

	if g.filtering {
		switch key.String() {
		case "enter":
			g.filtering = false
			g.filter.Blur()
		case "esc":
			g.filtering = false
			g.filter.SetValue("")
			g.filter.Blur()
		default:
			var cmd tea.Cmd
			g.filter, cmd = g.filter.Update(msg)
			return g, cmd
		}
		return g, nil
	}

	switch key.String() {
	case "q", "ctrl+c":
		return g, tea.Quit
	case "j", "down":
		if g.cursor < len(g.rows())-1 {
			g.cursor++
		}
	case "k", "up":
		if g.cursor > 0 {
			g.cursor--
		}
	case "g", "home":
		g.cursor = 0
	case "G", "end":
		g.cursor = len(g.rows()) - 1
	case "s":
		g.app.sortField = model.NextSortField(g.app.sortField)
	case "r":
		g.app.sortReversed = !g.app.sortReversed
		if !g.app.connOK {
			return g, g.app.retryNow()
		}
	case "f":
		g.filtering = true
		return g, g.filter.Focus()
	case "a":
		g.app.allContainers = !g.app.allContainers
		return g, fetchListCmd(g.app.client, g.app.allContainers)
	case "h", "?":
		return g, g.app.push(newHelpScreen())
	case "enter":
		if c := g.selected(); c != nil {
			return g, g.app.push(newDetailScreen(g.app, c.ID))
		}
	case "l":
		if c := g.selected(); c != nil {
			return g, g.app.push(newLogsScreen(g.app, c.ID))
		}
	case "e":
		if c := g.selected(); c != nil && c.Running() {
			return g, execShellCmd(g.app.client, g.app.cfg, c.ID)
		}
	case "S":
		if c := g.selected(); c != nil && !c.Running() {
			return g, g.app.push(newConfirmScreen(g.app, "start", c.ID))
		}
	case "X":
		if c := g.selected(); c != nil && c.Running() {
			return g, g.app.push(newConfirmScreen(g.app, "stop", c.ID))
		}
	case "K":
		if c := g.selected(); c != nil && c.Running() {
			return g, g.app.push(newConfirmScreen(g.app, "kill", c.ID))
		}
	case "D":
		if c := g.selected(); c != nil {
			return g, g.app.push(newConfirmScreen(g.app, "delete", c.ID))
		}
	}
	return g, nil
}

type column struct {
	title string
	width int
	cell  func(c *model.Container, now time.Time) string
}

func gridColumns(nameWidth int) []column {
	return []column{
		{"NAME", nameWidth, func(c *model.Container, _ time.Time) string { return c.ID }},
		{"STATE", 8, func(c *model.Container, _ time.Time) string { return c.State }},
		{"CPU", 7, func(c *model.Container, _ time.Time) string {
			if !c.Sample.HasRates {
				return "-"
			}
			return fmt.Sprintf("%.1f%%", c.Sample.CPUPercent)
		}},
		{"MEM", 12, func(c *model.Container, _ time.Time) string {
			if c.Sample.MemLimit == 0 {
				return "-"
			}
			return byteSize(c.Sample.MemUsage) + "/" + byteSize(c.Sample.MemLimit)
		}},
		{"NET RX/TX", 16, func(c *model.Container, _ time.Time) string {
			if !c.Sample.HasRates {
				return "-"
			}
			return rate(c.Sample.NetRxRate) + " / " + rate(c.Sample.NetTxRate)
		}},
		{"IO R/W", 16, func(c *model.Container, _ time.Time) string {
			if !c.Sample.HasRates {
				return "-"
			}
			return rate(c.Sample.IOReadRate) + " / " + rate(c.Sample.IOWriteRate)
		}},
		{"PIDS", 5, func(c *model.Container, _ time.Time) string {
			if c.Sample.PIDs == 0 {
				return "-"
			}
			return fmt.Sprintf("%d", c.Sample.PIDs)
		}},
		{"UPTIME", 8, func(c *model.Container, now time.Time) string {
			return shortDuration(c.Uptime(now))
		}},
		{"IP", 15, func(c *model.Container, _ time.Time) string {
			if c.IP == "" {
				return "-"
			}
			return c.IP
		}},
	}
}

func (g *gridScreen) view(width, height int) string {
	rows := g.rows()
	now := time.Now()

	nameWidth := 12
	for _, c := range rows {
		if len(c.ID) > nameWidth {
			nameWidth = len(c.ID)
		}
	}
	cols := gridColumns(nameWidth)

	var b strings.Builder
	head := make([]string, len(cols))
	for i, col := range cols {
		head[i] = pad(col.title, col.width)
	}
	b.WriteString(styleColHead.Render(strings.Join(head, "  ")))
	b.WriteString("\n")

	for i, c := range rows {
		cells := make([]string, len(cols))
		for j, col := range cols {
			text := pad(col.cell(c, now), col.width)
			switch col.title {
			case "STATE":
				text = stateStyle(c.State).Render(text)
			case "CPU":
				text = cpuStyle(c.Sample.CPUPercent).Render(text)
			}
			cells[j] = text
		}
		line := strings.Join(cells, "  ")
		if i == g.cursor {
			line = styleSelected.Render(strings.Join(stripStyles(cols, c, now), "  "))
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(rows) == 0 {
		b.WriteString(styleDim.Render("no containers"))
		b.WriteString("\n")
	}

	if g.filtering || g.filter.Value() != "" {
		b.WriteString("\n")
		b.WriteString("filter: " + g.filter.View())
		b.WriteString("\n")
	}
	return b.String()
}

// stripStyles renders a row without per-cell colors so the selection
// reverse-video style stays uniform.
func stripStyles(cols []column, c *model.Container, now time.Time) []string {
	cells := make([]string, len(cols))
	for j, col := range cols {
		cells[j] = pad(col.cell(c, now), col.width)
	}
	return cells
}

func pad(s string, w int) string {
	if len(s) > w {
		return s[:w-1] + "…"
	}
	return s + strings.Repeat(" ", w-len(s))
}
