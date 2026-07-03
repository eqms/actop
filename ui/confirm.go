package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// confirmScreen asks before running a lifecycle action.
type confirmScreen struct {
	app    *App
	action string
	id     string
}

func newConfirmScreen(app *App, action, id string) *confirmScreen {
	return &confirmScreen{app: app, action: action, id: id}
}

func (c *confirmScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return c, nil
	}
	switch key.String() {
	case "y", "enter":
		return nil, actionCmd(c.app.client, c.action, c.id)
	case "n", "esc", "q":
		return nil, nil
	}
	return c, nil
}

func (c *confirmScreen) view(width, height int) string {
	q := fmt.Sprintf("%s container %q?", c.action, c.id)
	return "\n  " + styleTitle.Render(q) + "\n\n  " +
		styleWarn.Render("[y]es") + "  " + styleDim.Render("[n]o") + "\n"
}
