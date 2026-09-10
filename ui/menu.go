package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/eqms/actop/model"
)

// menuItem is one entry in an action menu. key is its hotkey inside the
// menu, shown in front of the label as ctop does ("[l] log view"); an empty
// key makes the entry reachable by cursor only.
type menuItem struct {
	key   string
	label string
	build func(m *menuScreen) (screen, tea.Cmd)
}

// menuScreen is the ctop-style action menu, generic over its target: the
// items capture what they act on in closures at open time.
type menuScreen struct {
	app    *App
	title  string
	items  []menuItem
	cursor int
}

func newMenuScreen(app *App, title string, items []menuItem) *menuScreen {
	return &menuScreen{app: app, title: title, items: items}
}

// cancelItem closes the menu; ctop lists it as the last entry.
func cancelItem() menuItem {
	return menuItem{key: "c", label: "cancel", build: func(m *menuScreen) (screen, tea.Cmd) {
		return nil, nil
	}}
}

// newContainerMenu builds ctop's container menu: only the actions that apply
// to the container's current state are listed, each with ctop's hotkey.
// Start and restart run at once; everything that ends or destroys a running
// process still asks first.
func newContainerMenu(app *App, c *model.Container) *menuScreen {
	id := c.ID
	items := []menuItem{
		{key: "o", label: "single view", build: func(m *menuScreen) (screen, tea.Cmd) {
			return newDetailScreen(m.app, id), nil
		}},
		{key: "l", label: "log view", build: func(m *menuScreen) (screen, tea.Cmd) {
			// starts streaming itself: menu selection replaces the menu via
			// routeToTop instead of App.push (which normally calls start)
			l := newLogsScreen(m.app, id)
			return l, l.start()
		}},
	}
	if c.Running() {
		items = append(items,
			menuItem{key: "s", label: "stop", build: confirmItem("stop", id)},
			menuItem{key: "r", label: "restart", build: runItem("restart", id)},
			menuItem{key: "X", label: "stop + remove", build: confirmItem("remove", id)},
			menuItem{key: "e", label: "exec shell", build: func(m *menuScreen) (screen, tea.Cmd) {
				return nil, execShellCmd(m.app.client, m.app.cfg, id)
			}},
		)
		if url := c.WebURL(); url != "" {
			items = append(items, menuItem{key: "w", label: "open in browser", build: func(m *menuScreen) (screen, tea.Cmd) {
				return nil, openURLCmd(url)
			}})
		}
		items = append(items, menuItem{key: "K", label: "kill", build: confirmItem("kill", id)})
	} else {
		items = append(items,
			menuItem{key: "s", label: "start", build: runItem("start", id)},
			menuItem{key: "R", label: "remove", build: confirmItem("delete", id)},
		)
	}
	items = append(items, cancelItem())
	return newMenuScreen(app, c.ID, items)
}

func confirmItem(action, id string) func(*menuScreen) (screen, tea.Cmd) {
	return func(m *menuScreen) (screen, tea.Cmd) {
		return confirmAction(m.app, action, id), nil
	}
}

// runItem closes the menu and runs the action without a confirmation.
func runItem(action, id string) func(*menuScreen) (screen, tea.Cmd) {
	return func(m *menuScreen) (screen, tea.Cmd) {
		return nil, actionCmd(m.app.client, action, id)
	}
}

// newVolumeMenu and newImageMenu give the other tabs the same enter-menu.
func newVolumeMenu(app *App, name string) *menuScreen {
	items := []menuItem{
		{key: "R", label: "remove", build: func(m *menuScreen) (screen, tea.Cmd) {
			return volumeDeleteConfirm(m.app, name), nil
		}},
		cancelItem(),
	}
	return newMenuScreen(app, name, items)
}

func newImageMenu(app *App, ref string) *menuScreen {
	items := []menuItem{
		{key: "R", label: "remove", build: func(m *menuScreen) (screen, tea.Cmd) {
			return imageDeleteConfirm(m.app, ref), nil
		}},
		cancelItem(),
	}
	return newMenuScreen(app, ref, items)
}

// newSortMenu is ctop's sort selection: every field listed, the cursor on the
// active one, enter picks. The direction stays on r in the grid.
func newSortMenu(app *App) *menuScreen {
	items := make([]menuItem, 0, len(model.SortFields))
	cursor := 0
	for i, f := range model.SortFields {
		if f == app.sortField {
			cursor = i
		}
		items = append(items, menuItem{label: string(f), build: func(m *menuScreen) (screen, tea.Cmd) {
			m.app.sortField = f
			return nil, nil
		}})
	}
	m := newMenuScreen(app, "sort by", items)
	m.cursor = cursor
	return m
}

func (m *menuScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k := key.String(); k {
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
			if it.key != "" && it.key == k {
				return it.build(m)
			}
		}
	}
	return m, nil
}

// move shifts the cursor by delta, clamping at both ends.
func (m *menuScreen) move(delta int) {
	if i := m.cursor + delta; i >= 0 && i < len(m.items) {
		m.cursor = i
	}
}

// menuWidth is the entry column width inside the panel, hotkey included.
const menuWidth = 20

func (m *menuScreen) view(width, height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(safe(m.title)) + "\n\n")
	for i, it := range m.items {
		entry := it.label
		if it.key != "" {
			entry = "[" + it.key + "] " + it.label
		}
		entry = pad(entry, menuWidth)
		if i == m.cursor {
			b.WriteString("▸ " + styleSelected.Render(entry) + "\n")
		} else {
			b.WriteString("  " + entry + "\n")
		}
	}
	panel := styleMenuPanel.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}
