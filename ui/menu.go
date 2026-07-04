package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/eqms/actop/model"
)

// menuItem is one selectable action in the container menu.
// A nil build func marks the separator between navigation and
// destructive actions.
type menuItem struct {
	label  string
	hotkey string
	build  func(m *menuScreen) (screen, tea.Cmd)
}

// menuScreen is the ctop-style per-container action menu. It captures
// only the container ID; the item list is a state snapshot from open time.
type menuScreen struct {
	app    *App
	id     string
	items  []menuItem
	cursor int
}

// newMenuScreen builds the state-filtered item list. Running containers
// get the full action set, stopped ones only what applies to them.
func newMenuScreen(app *App, c *model.Container) *menuScreen {
	m := &menuScreen{app: app, id: c.ID}
	m.items = []menuItem{
		{label: "Details", hotkey: "enter", build: buildDetail},
		{label: "Logs", hotkey: "l", build: buildLogs},
	}
	if c.Running() {
		m.items = append(m.items, menuItem{label: "Shell", hotkey: "e", build: buildShell})
		if url := c.WebURL(); url != "" {
			m.items = append(m.items, menuItem{label: "Open URL", hotkey: "o",
				build: func(m *menuScreen) (screen, tea.Cmd) { return nil, openURLCmd(url) }})
		}
		m.items = append(m.items,
			menuItem{}, // separator
			menuItem{label: "Stop", hotkey: "X", build: buildConfirm("stop")},
			menuItem{label: "Kill", hotkey: "K", build: buildConfirm("kill")},
			menuItem{label: "Delete", hotkey: "D", build: buildConfirm("delete")},
		)
	} else {
		m.items = append(m.items,
			menuItem{}, // separator
			menuItem{label: "Start", hotkey: "S", build: buildConfirm("start")},
			menuItem{label: "Delete", hotkey: "D", build: buildConfirm("delete")},
		)
	}
	return m
}

func buildDetail(m *menuScreen) (screen, tea.Cmd) {
	return newDetailScreen(m.app, m.id), nil
}

// buildLogs starts streaming itself because the screen replaces the menu
// instead of going through App.push (which normally calls start).
func buildLogs(m *menuScreen) (screen, tea.Cmd) {
	l := newLogsScreen(m.app, m.id)
	return l, l.start()
}

func buildShell(m *menuScreen) (screen, tea.Cmd) {
	return nil, execShellCmd(m.app.client, m.app.cfg, m.id)
}

func buildConfirm(action string) func(*menuScreen) (screen, tea.Cmd) {
	return func(m *menuScreen) (screen, tea.Cmd) {
		return confirmAction(m.app, action, m.id), nil
	}
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
		return m.items[m.cursor].build(m)
	default:
		for _, it := range m.items {
			if it.build != nil && it.hotkey == key.String() {
				return it.build(m)
			}
		}
	}
	return m, nil
}

// move shifts the cursor by delta, skipping separators and clamping
// at both ends.
func (m *menuScreen) move(delta int) {
	i := m.cursor + delta
	for i >= 0 && i < len(m.items) && m.items[i].build == nil {
		i += delta
	}
	if i >= 0 && i < len(m.items) {
		m.cursor = i
	}
}

// view is completed in the rendering task; minimal placeholder so
// menuScreen satisfies the screen interface.
func (m *menuScreen) view(width, height int) string { return "" }
