package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/eqms/actop/model"
)

// menuItem is one entry in an action menu. Disabled entries stay visible
// but dimmed (ctop keeps the full list so the layout never shifts) and are
// skipped by the cursor.
type menuItem struct {
	label    string
	disabled bool
	build    func(m *menuScreen) (screen, tea.Cmd)
}

// menuScreen is the ctop-style action menu, generic over its target: the
// items capture what they act on in closures at open time.
type menuScreen struct {
	app    *App
	title  string
	items  []menuItem
	cursor int
}

// newMenuScreen places the cursor on the first enabled item.
func newMenuScreen(app *App, title string, items []menuItem) *menuScreen {
	m := &menuScreen{app: app, title: title, items: items}
	for i, it := range items {
		if !it.disabled {
			m.cursor = i
			break
		}
	}
	return m
}

// cancelItem closes the menu; ctop lists it as the last entry.
func cancelItem() menuItem {
	return menuItem{label: "Cancel", build: func(m *menuScreen) (screen, tea.Cmd) {
		return nil, nil
	}}
}

// newContainerMenu builds the fixed container action list. Entries that do
// not apply to the container's current state are disabled, not hidden.
func newContainerMenu(app *App, c *model.Container) *menuScreen {
	id := c.ID
	url := c.WebURL()
	running := c.Running()
	items := []menuItem{
		{label: "Details", build: func(m *menuScreen) (screen, tea.Cmd) {
			return newDetailScreen(m.app, id), nil
		}},
		{label: "Logs", build: func(m *menuScreen) (screen, tea.Cmd) {
			// starts streaming itself: menu selection replaces the menu via
			// routeToTop instead of App.push (which normally calls start)
			l := newLogsScreen(m.app, id)
			return l, l.start()
		}},
		{label: "Shell", disabled: !running, build: func(m *menuScreen) (screen, tea.Cmd) {
			return nil, execShellCmd(m.app.client, m.app.cfg, id)
		}},
		{label: "Open URL", disabled: !running || url == "", build: func(m *menuScreen) (screen, tea.Cmd) {
			return nil, openURLCmd(url)
		}},
		{label: "Start", disabled: running, build: confirmItem("start", id)},
		{label: "Stop", disabled: !running, build: confirmItem("stop", id)},
		{label: "Kill", disabled: !running, build: confirmItem("kill", id)},
		{label: "Delete", build: confirmItem("delete", id)},
		cancelItem(),
	}
	return newMenuScreen(app, c.ID, items)
}

func confirmItem(action, id string) func(*menuScreen) (screen, tea.Cmd) {
	return func(m *menuScreen) (screen, tea.Cmd) {
		return confirmAction(m.app, action, id), nil
	}
}

// newVolumeMenu and newImageMenu give the other tabs the same enter-menu.
func newVolumeMenu(app *App, name string) *menuScreen {
	items := []menuItem{
		{label: "Delete", build: func(m *menuScreen) (screen, tea.Cmd) {
			return volumeDeleteConfirm(m.app, name), nil
		}},
		cancelItem(),
	}
	return newMenuScreen(app, name, items)
}

func newImageMenu(app *App, ref string) *menuScreen {
	items := []menuItem{
		{label: "Delete", build: func(m *menuScreen) (screen, tea.Cmd) {
			return imageDeleteConfirm(m.app, ref), nil
		}},
		cancelItem(),
	}
	return newMenuScreen(app, ref, items)
}

func (m *menuScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "q":
		return nil, nil
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "enter":
		// cursor can only rest on enabled items; guard stays for safety
		if it := m.items[m.cursor]; !it.disabled {
			return it.build(m)
		}
	}
	return m, nil
}

// move shifts the cursor by delta, skipping disabled entries and clamping
// at both ends.
func (m *menuScreen) move(delta int) {
	i := m.cursor + delta
	for i >= 0 && i < len(m.items) && m.items[i].disabled {
		i += delta
	}
	if i >= 0 && i < len(m.items) {
		m.cursor = i
	}
}

// menuWidth is the label column width inside the panel.
const menuWidth = 16

func (m *menuScreen) view(width, height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(m.title) + "\n\n")
	for i, it := range m.items {
		label := pad(it.label, menuWidth)
		switch {
		case i == m.cursor:
			b.WriteString("▸ " + styleSelected.Render(label) + "\n")
		case it.disabled:
			b.WriteString("  " + styleDim.Render(label) + "\n")
		default:
			b.WriteString("  " + label + "\n")
		}
	}
	panel := styleMenuPanel.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}
