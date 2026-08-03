package ui

import tea "charm.land/bubbletea/v2"

// screen is one layer of the UI stack. The grid is the permanent base;
// detail, logs, confirm and help get pushed on top.
// update returns the replacement screen — returning nil pops the screen.
type screen interface {
	update(msg tea.Msg) (screen, tea.Cmd)
	view(width, height int) string
}
