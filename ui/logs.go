package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// maxLogLines bounds the in-memory log buffer (container logs has no --tail).
const maxLogLines = 2000

// logsScreen streams `container logs --follow` into a viewport.
type logsScreen struct {
	app     *App
	id      string
	session *logSession
	lines   []string
	vp      viewport.Model
	ready   bool
	ended   bool
	err     error
}

func newLogsScreen(app *App, id string) *logsScreen {
	return &logsScreen{app: app, id: id}
}

// start is called by App right after pushing the screen.
func (l *logsScreen) start() tea.Cmd {
	session, wait, err := startLogSession(l.app.client, l.id)
	if err != nil {
		l.ended, l.err = true, err
		return nil
	}
	l.session = session
	return wait
}

func (l *logsScreen) close() {
	if l.session != nil {
		l.session.cancel()
		l.session = nil
	}
}

func (l *logsScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "q", "esc":
			l.close()
			return nil, nil
		}
	case logLineMsg:
		atBottom := l.vp.AtBottom()
		l.lines = append(l.lines, m.line)
		if len(l.lines) > maxLogLines {
			l.lines = l.lines[len(l.lines)-maxLogLines:]
		}
		if l.ready {
			l.vp.SetContent(strings.Join(l.lines, "\n"))
			if atBottom {
				l.vp.GotoBottom()
			}
		}
		return l, waitForLogCmd(l.session.ch)
	case logStreamEndMsg:
		l.ended, l.err = true, m.err
		return l, nil
	case tea.WindowSizeMsg:
		l.ready = false
	}
	var cmd tea.Cmd
	l.vp, cmd = l.vp.Update(msg)
	return l, cmd
}

func (l *logsScreen) view(width, height int) string {
	if !l.ready {
		l.vp = viewport.New(width, max(height-2, 1))
		l.vp.SetContent(strings.Join(l.lines, "\n"))
		l.vp.GotoBottom()
		l.ready = true
	}
	status := styleDim.Render("logs: " + l.id + " · q/esc back · j/k scroll")
	if l.ended {
		suffix := "stream ended"
		if l.err != nil {
			suffix = "stream error: " + l.err.Error()
		}
		status = styleWarn.Render(suffix) + " " + status
	}
	return l.vp.View() + "\n" + status
}
