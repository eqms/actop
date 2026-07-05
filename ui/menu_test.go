package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/model"
)

var fixedLabels = []string{"Details", "Logs", "Shell", "Open URL", "Start", "Stop", "Kill", "Delete", "Cancel"}

func menuLabels(m *menuScreen) []string {
	var out []string
	for _, it := range m.items {
		out = append(out, it.label)
	}
	return out
}

func disabledLabels(m *menuScreen) map[string]bool {
	out := map[string]bool{}
	for _, it := range m.items {
		if it.disabled {
			out[it.label] = true
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

func runningContainer(ports ...cli.PublishedPort) *model.Container {
	return &model.Container{ID: "db", State: "running", Ports: ports}
}

func stoppedContainer() *model.Container {
	return &model.Container{ID: "db", State: "stopped"}
}

func TestContainerMenuFixedList(t *testing.T) {
	app, _ := testApp()
	for name, c := range map[string]*model.Container{
		"running": runningContainer(),
		"stopped": stoppedContainer(),
	} {
		m := newContainerMenu(app, c)
		if got := menuLabels(m); !equalStrings(got, fixedLabels) {
			t.Errorf("%s menu = %v, want fixed %v", name, got, fixedLabels)
		}
	}
}

func TestContainerMenuEnabledRunning(t *testing.T) {
	app, _ := testApp()
	withPort := newContainerMenu(app, runningContainer(cli.PublishedPort{Proto: "tcp", HostPort: 8080}))
	if d := disabledLabels(withPort); len(d) != 1 || !d["Start"] {
		t.Errorf("running+port disabled = %v, want only Start", d)
	}
	noPort := newContainerMenu(app, runningContainer())
	if d := disabledLabels(noPort); len(d) != 2 || !d["Start"] || !d["Open URL"] {
		t.Errorf("running w/o port disabled = %v, want Start + Open URL", d)
	}
}

func TestContainerMenuEnabledStopped(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, stoppedContainer())
	want := map[string]bool{"Shell": true, "Open URL": true, "Stop": true, "Kill": true}
	got := disabledLabels(m)
	if len(got) != len(want) {
		t.Fatalf("stopped disabled = %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("%s must be disabled when stopped", k)
		}
	}
}

func TestMenuCursorStartsOnFirstEnabled(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, stoppedContainer())
	if m.items[m.cursor].label != "Details" {
		t.Errorf("cursor starts on %q, want Details", m.items[m.cursor].label)
	}
}

func TestMenuMoveSkipsDisabledAndClamps(t *testing.T) {
	app, _ := testApp()
	// stopped: indices 0 Details, 1 Logs, 2 Shell(d), 3 Open URL(d),
	// 4 Start, 5 Stop(d), 6 Kill(d), 7 Delete, 8 Cancel
	m := newContainerMenu(app, stoppedContainer())
	m.update(key('k')) // clamp at top
	if m.items[m.cursor].label != "Details" {
		t.Errorf("after k at top: %q, want Details", m.items[m.cursor].label)
	}
	m.update(key('j'))
	m.update(key('j')) // skips Shell + Open URL
	if m.items[m.cursor].label != "Start" {
		t.Errorf("cursor = %q, want Start (disabled skipped)", m.items[m.cursor].label)
	}
	m.update(key('j')) // skips Stop + Kill
	if m.items[m.cursor].label != "Delete" {
		t.Errorf("cursor = %q, want Delete", m.items[m.cursor].label)
	}
	for i := 0; i < 5; i++ {
		m.update(key('j'))
	}
	if m.items[m.cursor].label != "Cancel" {
		t.Errorf("cursor = %q, want Cancel (clamped at end)", m.items[m.cursor].label)
	}
	m.update(key('k'))
	if m.items[m.cursor].label != "Delete" {
		t.Errorf("cursor = %q, want Delete", m.items[m.cursor].label)
	}
}

func TestMenuEnterOnDetails(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	next, _ := m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := next.(*detailScreen); !ok {
		t.Errorf("enter on Details = %T, want *detailScreen", next)
	}
}

func TestMenuEnterOnStopReturnsConfirm(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	for m.items[m.cursor].label != "Stop" {
		m.update(key('j'))
	}
	next, _ := m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := next.(*confirmScreen); !ok {
		t.Errorf("enter on Stop = %T, want *confirmScreen", next)
	}
}

func TestMenuCancelCloses(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	for m.items[m.cursor].label != "Cancel" {
		m.update(key('j'))
	}
	next, cmd := m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if next != nil || cmd != nil {
		t.Errorf("Cancel = (%T, %v), want (nil, nil)", next, cmd)
	}
}

func TestMenuEscCloses(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	next, cmd := m.update(tea.KeyMsg{Type: tea.KeyEsc})
	if next != nil || cmd != nil {
		t.Errorf("esc = (%T, %v), want (nil, nil)", next, cmd)
	}
}

func TestMenuNoHotkeyDispatch(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	next, cmd := m.update(key('X'))
	if next != screen(m) || cmd != nil {
		t.Errorf("X inside menu = (%T, %v), want menu unchanged (no hotkey dispatch)", next, cmd)
	}
	if _, ok := next.(*confirmScreen); ok {
		t.Error("X inside menu must not open a confirm")
	}
}

func TestMenuViewPlainList(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	out := m.view(80, 24)
	for _, want := range fixedLabels {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q", want)
		}
	}
	if !strings.Contains(out, "db") {
		t.Error("view missing title")
	}
	marker := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "▸") {
			marker++
			if !strings.Contains(line, "Details") {
				t.Errorf("marker on %q, want the Details line", line)
			}
		}
	}
	if marker != 1 {
		t.Errorf("marker count = %d, want 1", marker)
	}
	if strings.Contains(out, "select · enter · esc") {
		t.Error("footer hint must be removed")
	}
}

func TestEnterOnGridOpensMenu(t *testing.T) {
	app, _ := testApp()
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID:     "db",
		Status: &cli.Status{State: "running"},
	}}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := app.top().(*menuScreen)
	if !ok {
		t.Fatalf("expected menu screen on top, got %T", app.top())
	}
	if m.title != "db" {
		t.Errorf("menu title = %q, want db", m.title)
	}
}

func TestMenuStopFlowCallsClient(t *testing.T) {
	app, mock := testApp()
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID:     "db",
		Status: &cli.Status{State: "running"},
	}}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := app.top().(*menuScreen)
	for m.items[m.cursor].label != "Stop" {
		app.Update(key('j'))
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := app.top().(*confirmScreen); !ok {
		t.Fatalf("expected confirm screen, got %T", app.top())
	}
	_, cmd := app.Update(key('y'))
	if cmd == nil {
		t.Fatal("expected action command from confirm")
	}
	cmd()
	found := false
	for _, c := range mock.Calls {
		if c.Method == "Stop" && c.ID == "db" {
			found = true
		}
	}
	if !found {
		t.Errorf("Stop not called via menu flow: %+v", mock.Calls)
	}
	if _, ok := app.top().(*gridScreen); !ok {
		t.Errorf("stack not back on grid, top is %T", app.top())
	}
}

func TestMenuEscReturnsToGrid(t *testing.T) {
	app, mock := testApp()
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID:     "db",
		Status: &cli.Status{State: "running"},
	}}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := app.top().(*gridScreen); !ok {
		t.Fatalf("expected grid after esc, got %T", app.top())
	}
	if len(mock.Calls) != 0 {
		t.Errorf("no client calls expected, got %+v", mock.Calls)
	}
}

func TestVolumeMenuDeleteFlow(t *testing.T) {
	app, mock := testApp()
	app.Update(key('2'))
	app.Update(volumesResultMsg{volumes: []cli.Volume{{
		ID:            "vol1",
		Configuration: cli.VolumeConfig{Name: "vol1"},
	}}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := app.top().(*menuScreen)
	if !ok {
		t.Fatalf("expected menu on volumes tab, got %T", app.top())
	}
	if m.title != "vol1" {
		t.Errorf("menu title = %q, want vol1", m.title)
	}
	if got := menuLabels(m); !equalStrings(got, []string{"Delete", "Cancel"}) {
		t.Fatalf("volume menu = %v, want [Delete Cancel]", got)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Delete -> confirm
	c, ok := app.top().(*confirmScreen)
	if !ok {
		t.Fatalf("expected confirm screen, got %T", app.top())
	}
	if !strings.Contains(c.question, "data is lost") {
		t.Errorf("confirm question %q must keep the data-loss warning", c.question)
	}
	_, cmd := app.Update(key('y'))
	if cmd == nil {
		t.Fatal("expected delete command")
	}
	cmd()
	found := false
	for _, call := range mock.Calls {
		if call.Method == "VolumeDelete" && call.ID == "vol1" {
			found = true
		}
	}
	if !found {
		t.Errorf("VolumeDelete not called via menu: %+v", mock.Calls)
	}
	if _, ok := app.top().(*volumesScreen); !ok {
		t.Errorf("stack not back on volumes tab, top is %T", app.top())
	}
}

func TestVolumeMenuCancel(t *testing.T) {
	app, mock := testApp()
	app.Update(key('2'))
	app.Update(volumesResultMsg{volumes: []cli.Volume{{
		ID:            "vol1",
		Configuration: cli.VolumeConfig{Name: "vol1"},
	}}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app.Update(key('j')) // cursor to Cancel
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := app.top().(*volumesScreen); !ok {
		t.Fatalf("expected volumes tab after Cancel, got %T", app.top())
	}
	if len(mock.Calls) != 0 {
		t.Errorf("no client calls expected, got %+v", mock.Calls)
	}
}

func TestImageMenuDeleteFlow(t *testing.T) {
	app, mock := testApp()
	app.Update(key('3'))
	app.Update(imagesResultMsg{images: []cli.Image{{
		ID:            "img1",
		Configuration: cli.ImageConfig{Name: "docker.io/library/postgres:17"},
	}}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := app.top().(*menuScreen)
	if !ok {
		t.Fatalf("expected menu on images tab, got %T", app.top())
	}
	if m.title != "docker.io/library/postgres:17" {
		t.Errorf("menu title = %q, want image ref", m.title)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Delete -> confirm
	if _, ok := app.top().(*confirmScreen); !ok {
		t.Fatalf("expected confirm screen, got %T", app.top())
	}
	_, cmd := app.Update(key('y'))
	if cmd == nil {
		t.Fatal("expected delete command")
	}
	cmd()
	found := false
	for _, call := range mock.Calls {
		if call.Method == "ImageDelete" && call.ID == "docker.io/library/postgres:17" {
			found = true
		}
	}
	if !found {
		t.Errorf("ImageDelete not called via menu: %+v", mock.Calls)
	}
}
