package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// columnsScreen toggles grid columns for the session. Persist a selection
// via `columns = [...]` in the config file.
type columnsScreen struct {
	app    *App
	cursor int
}

func newColumnsScreen(app *App) *columnsScreen {
	return &columnsScreen{app: app}
}

func (s *columnsScreen) enabled(key string) bool {
	for _, k := range s.app.columns {
		if k == key {
			return true
		}
	}
	return false
}

// toggle flips a column while keeping registry order; "name" is locked.
func (s *columnsScreen) toggle(key string) {
	if key == "name" {
		return
	}
	if s.enabled(key) {
		out := make([]string, 0, len(s.app.columns))
		for _, k := range s.app.columns {
			if k != key {
				out = append(out, k)
			}
		}
		s.app.columns = out
		return
	}
	out := make([]string, 0, len(s.app.columns)+1)
	for _, k := range allColumnKeys {
		if k == key || s.enabled(k) {
			out = append(out, k)
		}
	}
	s.app.columns = out
}

func (s *columnsScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "q", "esc", "enter", "c":
		return nil, nil
	case "j", "down":
		if s.cursor < len(allColumnKeys)-1 {
			s.cursor++
		}
	case "k", "up":
		if s.cursor > 0 {
			s.cursor--
		}
	case " ":
		s.toggle(allColumnKeys[s.cursor])
	}
	return s, nil
}

func (s *columnsScreen) view(width, height int) string {
	var b strings.Builder
	b.WriteString("\n  " + styleTitle.Render("grid columns") + "\n\n")
	for i, key := range allColumnKeys {
		mark := "[ ]"
		if s.enabled(key) {
			mark = "[x]"
		}
		label := "  " + mark + " " + key
		if key == "name" {
			label += styleDim.Render(" (always on)")
		}
		if i == s.cursor {
			label = styleSelected.Render(label)
		}
		b.WriteString(label + "\n")
	}
	b.WriteString("\n  " + styleDim.Render("space toggle · j/k move · q close · persist via config.toml columns=[...]") + "\n")
	return b.String()
}
