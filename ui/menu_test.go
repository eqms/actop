package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/model"
)

func menuLabels(m *menuScreen) []string {
	var out []string
	for _, it := range m.items {
		if it.build != nil {
			out = append(out, it.label)
		}
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMenuItemsRunning(t *testing.T) {
	app, _ := testApp()
	c := &model.Container{ID: "db", State: "running"}
	m := newMenuScreen(app, c)
	want := []string{"Details", "Logs", "Shell", "Stop", "Kill", "Delete"}
	if got := menuLabels(m); !equalStrings(got, want) {
		t.Errorf("running menu = %v, want %v", got, want)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (Details)", m.cursor)
	}
	if m.id != "db" {
		t.Errorf("id = %q, want db", m.id)
	}
}

func TestMenuItemsStopped(t *testing.T) {
	app, _ := testApp()
	c := &model.Container{ID: "db", State: "stopped"}
	m := newMenuScreen(app, c)
	want := []string{"Details", "Logs", "Start", "Delete"}
	if got := menuLabels(m); !equalStrings(got, want) {
		t.Errorf("stopped menu = %v, want %v", got, want)
	}
}

func TestMenuOpenURLOnlyWithPort(t *testing.T) {
	app, _ := testApp()
	withPort := &model.Container{ID: "web", State: "running",
		Ports: []cli.PublishedPort{{Proto: "tcp", HostPort: 8080}}}
	m := newMenuScreen(app, withPort)
	want := []string{"Details", "Logs", "Shell", "Open URL", "Stop", "Kill", "Delete"}
	if got := menuLabels(m); !equalStrings(got, want) {
		t.Errorf("menu with port = %v, want %v", got, want)
	}

	noPort := &model.Container{ID: "db", State: "running"}
	for _, it := range newMenuScreen(app, noPort).items {
		if it.label == "Open URL" {
			t.Error("Open URL must not appear without a WebURL")
		}
	}
}

func TestMenuHasSeparatorBeforeDestructive(t *testing.T) {
	app, _ := testApp()
	m := newMenuScreen(app, &model.Container{ID: "db", State: "running"})
	sep := -1
	for i, it := range m.items {
		if it.build == nil {
			sep = i
		}
	}
	if sep == -1 {
		t.Fatal("running menu must contain a separator")
	}
	if m.items[sep+1].label != "Stop" {
		t.Errorf("item after separator = %q, want Stop", m.items[sep+1].label)
	}
}

func TestMenuNavigationSkipsSeparatorAndClamps(t *testing.T) {
	app, _ := testApp()
	m := newMenuScreen(app, &model.Container{ID: "db", State: "running"})
	// items: 0 Details, 1 Logs, 2 Shell, 3 separator, 4 Stop, 5 Kill, 6 Delete
	m.update(key('k')) // clamp at top
	if m.cursor != 0 {
		t.Errorf("cursor after k at top = %d, want 0", m.cursor)
	}
	m.update(key('j'))
	m.update(key('j'))
	m.update(key('j')) // must skip separator, land on Stop
	if m.items[m.cursor].label != "Stop" {
		t.Errorf("cursor on %q, want Stop (separator skipped)", m.items[m.cursor].label)
	}
	for i := 0; i < 10; i++ {
		m.update(key('j'))
	}
	if m.items[m.cursor].label != "Delete" {
		t.Errorf("cursor on %q, want Delete (clamped at end)", m.items[m.cursor].label)
	}
	m.update(key('k')) // back up over items, later over the separator
	if m.items[m.cursor].label != "Kill" {
		t.Errorf("cursor on %q, want Kill", m.items[m.cursor].label)
	}
}

func TestMenuEnterOnDetails(t *testing.T) {
	app, _ := testApp()
	m := newMenuScreen(app, &model.Container{ID: "db", State: "running"})
	next, _ := m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := next.(*detailScreen); !ok {
		t.Errorf("enter on Details = %T, want *detailScreen", next)
	}
}

func TestMenuEnterOnStopReturnsConfirm(t *testing.T) {
	app, _ := testApp()
	m := newMenuScreen(app, &model.Container{ID: "db", State: "running"})
	m.update(key('j'))
	m.update(key('j'))
	m.update(key('j')) // Stop
	next, _ := m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := next.(*confirmScreen); !ok {
		t.Errorf("enter on Stop = %T, want *confirmScreen", next)
	}
}

func TestMenuHotkeySelectsItem(t *testing.T) {
	app, _ := testApp()
	m := newMenuScreen(app, &model.Container{ID: "db", State: "running"})
	next, _ := m.update(key('D'))
	if _, ok := next.(*confirmScreen); !ok {
		t.Errorf("hotkey D = %T, want *confirmScreen", next)
	}
}

func TestMenuEscCloses(t *testing.T) {
	app, _ := testApp()
	m := newMenuScreen(app, &model.Container{ID: "db", State: "running"})
	next, cmd := m.update(tea.KeyMsg{Type: tea.KeyEsc})
	if next != nil || cmd != nil {
		t.Errorf("esc = (%T, %v), want (nil, nil)", next, cmd)
	}
}

func TestMenuViewRendersItemsAndTitle(t *testing.T) {
	app, _ := testApp()
	m := newMenuScreen(app, &model.Container{ID: "db", State: "running"})
	out := m.view(80, 24)
	for _, want := range []string{"db", "Details", "Logs", "Shell", "Stop", "Kill", "Delete"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q", want)
		}
	}

	// The cursor marker must appear on exactly one line, and that line
	// must be the initially selected "Details" row.
	lines := strings.Split(out, "\n")
	markerLines := 0
	for _, line := range lines {
		if strings.Contains(line, "▸") {
			markerLines++
			if !strings.Contains(line, "Details") {
				t.Errorf("cursor marker line = %q, want it to contain %q", line, "Details")
			}
		}
	}
	if markerLines != 1 {
		t.Errorf("cursor marker lines = %d, want exactly 1", markerLines)
	}

	if !strings.Contains(out, "↑/↓ select · enter · esc") {
		t.Error("view missing footer hint")
	}

	if !strings.Contains(out, strings.Repeat("─", 10)) {
		t.Error("view missing separator rule")
	}

	if !strings.Contains(out, "╭") {
		t.Error("view missing rounded panel border")
	}

	if !strings.Contains(out, "enter") {
		t.Error("view missing enter hotkey text")
	}

	stopFound, deleteFound := false, false
	for _, line := range lines {
		if strings.Contains(line, "Stop") {
			stopFound = true
			if !strings.Contains(line, "X") {
				t.Errorf("Stop line = %q, want it to contain hotkey %q", line, "X")
			}
		}
		if strings.Contains(line, "Delete") {
			deleteFound = true
			if !strings.Contains(line, "D") {
				t.Errorf("Delete line = %q, want it to contain hotkey %q", line, "D")
			}
		}
	}
	if !stopFound {
		t.Error("view missing Stop line")
	}
	if !deleteFound {
		t.Error("view missing Delete line")
	}
}
