package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

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
	key, ok := msg.(tea.KeyPressMsg)
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
		return g, g.app.push(newSortMenu(g.app))
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
			return g, g.app.push(newContainerMenu(g.app, c))
		}
	case "l":
		if c := g.selected(); c != nil {
			return g, g.app.push(newLogsScreen(g.app, c.ID))
		}
	case "e":
		if c := g.selected(); c != nil && c.Running() {
			return g, execShellCmd(g.app.client, g.app.cfg, c.ID)
		}
	case "o":
		if c := g.selected(); c != nil && c.Running() {
			if url := c.WebURL(); url != "" {
				return g, openURLCmd(url)
			}
			g.app.statusMsg = "no published port or IP to open"
		}
	case "c":
		return g, g.app.push(newColumnsScreen(g.app))
	case "S":
		if c := g.selected(); c != nil && !c.Running() {
			return g, actionCmd(g.app.client, "start", c.ID)
		}
	case "X":
		if c := g.selected(); c != nil && c.Running() {
			return g, g.app.push(confirmAction(g.app, "stop", c.ID))
		}
	case "K":
		if c := g.selected(); c != nil && c.Running() {
			return g, g.app.push(confirmAction(g.app, "kill", c.ID))
		}
	case "D":
		if c := g.selected(); c != nil {
			// a running container cannot be deleted without stopping it
			if c.Running() {
				return g, g.app.push(confirmAction(g.app, "remove", c.ID))
			}
			return g, g.app.push(confirmAction(g.app, "delete", c.ID))
		}
	}
	return g, nil
}

type column struct {
	key   string
	title string
	width int
	cell  func(c *model.Container, now time.Time) string
}

// allColumnKeys is the registry order; config `columns` and the toggle
// screen (key c) select and order a subset. "name" is always shown.
var allColumnKeys = []string{"name", "state", "cpu", "hist", "mem", "net", "io", "pids", "uptime", "ip"}

func allColumns(nameWidth int) map[string]column {
	cols := map[string]column{}
	for _, c := range gridColumnDefs(nameWidth) {
		cols[c.key] = c
	}
	return cols
}

// gridColumns resolves the enabled keys (in order) to column definitions.
func gridColumns(enabled []string, nameWidth int) []column {
	defs := allColumns(nameWidth)
	out := make([]column, 0, len(enabled))
	for _, key := range enabled {
		if col, ok := defs[key]; ok {
			out = append(out, col)
		}
	}
	return out
}

func gridColumnDefs(nameWidth int) []column {
	return []column{
		{"name", "NAME", nameWidth, func(c *model.Container, _ time.Time) string { return c.ID }},
		{"state", "STATE", 8, func(c *model.Container, _ time.Time) string { return c.State }},
		{"cpu", "CPU", 7, func(c *model.Container, _ time.Time) string {
			if !c.Sample.HasRates {
				return "-"
			}
			return fmt.Sprintf("%.1f%%", c.Sample.CPUPercent)
		}},
		{"hist", "HIST", 12, func(c *model.Container, _ time.Time) string {
			if c.History == nil || c.History.CPU.Len() == 0 {
				return "-"
			}
			return sparkline(c.History.CPU.Values(), 12, 10)
		}},
		{"mem", "MEM", 12, func(c *model.Container, _ time.Time) string {
			if c.Sample.MemLimit == 0 {
				return "-"
			}
			return byteSize(c.Sample.MemUsage) + "/" + byteSize(c.Sample.MemLimit)
		}},
		{"net", "NET RX/TX", 16, func(c *model.Container, _ time.Time) string {
			if !c.Sample.HasRates {
				return "-"
			}
			return rate(c.Sample.NetRxRate) + " / " + rate(c.Sample.NetTxRate)
		}},
		{"io", "IO R/W", 16, func(c *model.Container, _ time.Time) string {
			if !c.Sample.HasRates {
				return "-"
			}
			return rate(c.Sample.IOReadRate) + " / " + rate(c.Sample.IOWriteRate)
		}},
		{"pids", "PIDS", 5, func(c *model.Container, _ time.Time) string {
			if c.Sample.PIDs == 0 {
				return "-"
			}
			return fmt.Sprintf("%d", c.Sample.PIDs)
		}},
		{"uptime", "UPTIME", 8, func(c *model.Container, now time.Time) string {
			return shortDuration(c.Uptime(now))
		}},
		{"ip", "IP", 15, func(c *model.Container, _ time.Time) string {
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
		if w := displayWidth(c.ID); w > nameWidth {
			nameWidth = w
		}
	}
	cols := gridColumns(g.app.columns, nameWidth)

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

// pad sanitizes s and fits it to w cells. It is the choke point for every
// table cell, so CLI-supplied text cannot reach the terminal unfiltered.
// Rune-aware: sparkline glyphs and umlauts are multi-byte.
func pad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(safe(s))
	if len(r) > w {
		return string(r[:w-1]) + "…"
	}
	return string(r) + strings.Repeat(" ", w-len(r))
}

// confirmAction wraps a container lifecycle action in a confirm dialog.
// The prompt shows the sanitized ID -- a name padded with escape sequences
// must not obscure which container is about to be destroyed -- while the
// command still targets the real one.
func confirmAction(app *App, action, id string) *confirmScreen {
	verb := action
	if action == "remove" {
		verb = "stop and delete"
	}
	q := fmt.Sprintf("%s container %q?", verb, safe(id))
	return newConfirmScreen(q, actionCmd(app.client, action, id))
}

// capturesInput reports whether the grid is in filter-typing mode, so the
// app must not hijack keys like 1/2/3 for tab switching.
func (g *gridScreen) capturesInput() bool { return g.filtering }
