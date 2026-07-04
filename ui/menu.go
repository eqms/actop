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
