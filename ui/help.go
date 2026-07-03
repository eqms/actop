package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type helpScreen struct{}

func newHelpScreen() *helpScreen { return &helpScreen{} }

func (h *helpScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return nil, nil
	}
	return h, nil
}

var helpEntries = [][2]string{
	{"enter", "container details"},
	{"l", "logs (follow)"},
	{"e", "exec shell"},
	{"S", "start container"},
	{"X", "stop container"},
	{"K", "kill container"},
	{"D", "delete container"},
	{"s", "cycle sort field"},
	{"r", "reverse sort / retry connection"},
	{"f", "filter"},
	{"a", "toggle stopped containers"},
	{"j/k", "navigate"},
	{"h/?", "this help"},
	{"q", "quit / back"},
}

func (h *helpScreen) view(width, height int) string {
	var b strings.Builder
	b.WriteString("\n  " + styleTitle.Render("actop keys") + "\n\n")
	for _, e := range helpEntries {
		b.WriteString("  " + styleColHead.Render(pad(e[0], 7)) + e[1] + "\n")
	}
	b.WriteString("\n  " + styleDim.Render("press any key to close") + "\n")
	return b.String()
}
