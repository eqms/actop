package ui

import (
	tea "charm.land/bubbletea/v2"
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
	key, ok := msg.(tea.KeyPressMsg)
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
	// question embeds a container/volume/image name chosen by whoever
	// created it, so it is sanitized like any other CLI-supplied text.
	return "\n  " + styleTitle.Render(safe(c.question)) + "\n\n  " +
		styleWarn.Render("[y]es") + "  " + styleDim.Render("[n]o") + "\n"
}
