package ui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
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
	case tea.KeyPressMsg:
		switch m.String() {
		case "q", "esc":
			l.close()
			return nil, nil
		}
	case logLineMsg:
		atBottom := l.vp.AtBottom()
		// Container stdout is untrusted: strip escape sequences and control
		// chars before the line reaches the terminal renderer.
		l.lines = append(l.lines, safe(m.line))
		if len(l.lines) > maxLogLines {
			l.lines = l.lines[len(l.lines)-maxLogLines:]
		}
		if l.ready {
			l.vp.SetContent(strings.Join(l.lines, "\n"))
			if atBottom {
				l.vp.GotoBottom()
			}
		}
		if l.session == nil {
			// close() ran while this line was in flight; keep the text, but
			// do not wait on a channel that no longer has a producer
			return l, nil
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
		l.vp = viewport.New()
		l.vp.SetWidth(width)
		l.vp.SetHeight(max(height-2, 1))
		l.vp.SetContent(strings.Join(l.lines, "\n"))
		l.vp.GotoBottom()
		l.ready = true
	}
	status := styleDim.Render("logs: " + safe(l.id) + " · q/esc back · j/k scroll")
	if l.ended {
		suffix := "stream ended"
		if l.err != nil {
			// the error text can quote CLI stderr, which the container writes
			suffix = "stream error: " + safe(l.err.Error())
		}
		status = styleWarn.Render(suffix) + " " + status
	}
	return l.vp.View() + "\n" + status
}
