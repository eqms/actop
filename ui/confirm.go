package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// confirmScreen asks before running a destructive command.
type confirmScreen struct {
	question string
	cmd      tea.Cmd
}

func newConfirmScreen(question string, cmd tea.Cmd) *confirmScreen {
	return &confirmScreen{question: question, cmd: cmd}
}

func (c *confirmScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return c, nil
	}
	switch key.String() {
	case "y", "enter":
		return nil, c.cmd
	case "n", "esc", "q":
		return nil, nil
	}
	return c, nil
}

func (c *confirmScreen) view(width, height int) string {
	return "\n  " + styleTitle.Render(c.question) + "\n\n  " +
		styleWarn.Render("[y]es") + "  " + styleDim.Render("[n]o") + "\n"
}
