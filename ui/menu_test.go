package ui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/model"
)

// menuEntries renders the items the way the panel shows them, so the tests
// pin both the hotkey and the label.
func menuEntries(m *menuScreen) []string {
	var out []string
	for _, it := range m.items {
		out = append(out, "["+it.key+"] "+it.label)
	}
	return out
}

func menuLabels(m *menuScreen) []string {
	var out []string
	for _, it := range m.items {
		out = append(out, it.label)
	}
	return out
}

// calls flattens the recorded client calls to "Method id" for order checks.
func calls(mock *cli.MockClient) []string {
	var out []string
	for _, c := range mock.CallList() {
		out = append(out, c.Method+" "+c.ID)
	}
	return out
}

func runningContainer(ports ...cli.PublishedPort) *model.Container {
	return &model.Container{ID: "db", State: "running", Ports: ports}
}

func stoppedContainer() *model.Container {
	return &model.Container{ID: "db", State: "stopped"}
}

func TestContainerMenuRunning(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer(cli.PublishedPort{Proto: "tcp", HostPort: 8080}))
	want := []string{"[o] single view", "[l] log view", "[s] stop", "[r] restart",
		"[X] stop + remove", "[e] exec shell", "[w] open in browser", "[K] kill", "[c] cancel"}
	if got := menuEntries(m); !slices.Equal(got, want) {
		t.Errorf("running menu = %v, want %v", got, want)
	}
}

func TestContainerMenuRunningWithoutURL(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	for _, e := range menuEntries(m) {
		if e == "[w] open in browser" {
			t.Error("browser entry offered without a published port or IP")
		}
	}
}

func TestContainerMenuStopped(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, stoppedContainer())
	want := []string{"[o] single view", "[l] log view", "[s] start", "[R] remove", "[c] cancel"}
	if got := menuEntries(m); !slices.Equal(got, want) {
		t.Errorf("stopped menu = %v, want %v", got, want)
	}
}

// A hotkey that collides with a navigation key would make that entry
// unreachable by cursor or fire it while the user only meant to move.
func TestMenuKeysDoNotShadowNavigation(t *testing.T) {
	app, _ := testApp()
	reserved := map[string]bool{"j": true, "k": true, "q": true, "up": true,
		"down": true, "enter": true, "esc": true}
	menus := map[string]*menuScreen{
		"running": newContainerMenu(app, runningContainer(cli.PublishedPort{Proto: "tcp", HostPort: 80})),
		"stopped": newContainerMenu(app, stoppedContainer()),
		"volume":  newVolumeMenu(app, "vol1"),
		"image":   newImageMenu(app, "img"),
	}
	for name, m := range menus {
		seen := map[string]bool{}
		for _, it := range m.items {
			if reserved[it.key] {
				t.Errorf("%s menu: %q uses navigation key %q", name, it.label, it.key)
			}
			if seen[it.key] {
				t.Errorf("%s menu: hotkey %q assigned twice", name, it.key)
			}
			seen[it.key] = true
		}
	}
}

func TestMenuCursorStartsOnFirstEntry(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, stoppedContainer())
	if m.items[m.cursor].label != "single view" {
		t.Errorf("cursor starts on %q, want single view", m.items[m.cursor].label)
	}
}

func TestMenuMoveClamps(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, stoppedContainer()) // 5 entries
	m.update(key('k'))
	if m.cursor != 0 {
		t.Errorf("after k at top: cursor %d, want 0", m.cursor)
	}
	for i := 0; i < 10; i++ {
		m.update(key('j'))
	}
	if m.items[m.cursor].label != "cancel" {
		t.Errorf("cursor = %q, want cancel (clamped at end)", m.items[m.cursor].label)
	}
	m.update(key('k'))
	if m.items[m.cursor].label != "remove" {
		t.Errorf("cursor = %q, want remove", m.items[m.cursor].label)
	}
}

func TestMenuEnterOnSingleView(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	next, _ := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if _, ok := next.(*detailScreen); !ok {
		t.Errorf("enter on single view = %T, want *detailScreen", next)
	}
}

func TestMenuHotkeyOpensSingleView(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	m.update(key('j')) // cursor elsewhere: the hotkey must not depend on it
	next, _ := m.update(key('o'))
	if _, ok := next.(*detailScreen); !ok {
		t.Errorf("o inside menu = %T, want *detailScreen", next)
	}
}

func TestMenuHotkeyStopConfirms(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	next, _ := m.update(key('s'))
	c, ok := next.(*confirmScreen)
	if !ok {
		t.Fatalf("s on running = %T, want *confirmScreen", next)
	}
	if !strings.Contains(c.question, "stop container") {
		t.Errorf("question = %q, want a stop confirmation", c.question)
	}
}

func TestMenuStartRunsWithoutConfirm(t *testing.T) {
	app, mock := testApp()
	m := newContainerMenu(app, stoppedContainer())
	next, cmd := m.update(key('s'))
	if next != nil || cmd == nil {
		t.Fatalf("s on stopped = (%T, cmd nil=%v), want menu closed and a command", next, cmd == nil)
	}
	done, ok := cmd().(actionDoneMsg)
	if !ok || done.action != "start" || done.err != nil {
		t.Errorf("result = %+v, want start ok", done)
	}
	if got := calls(mock); !slices.Equal(got, []string{"Start db"}) {
		t.Errorf("calls = %v, want [Start db]", got)
	}
}

func TestMenuRestartStopsThenStarts(t *testing.T) {
	app, mock := testApp()
	m := newContainerMenu(app, runningContainer())
	next, cmd := m.update(key('r'))
	if next != nil || cmd == nil {
		t.Fatalf("r = (%T, cmd nil=%v), want menu closed and a command", next, cmd == nil)
	}
	done := cmd().(actionDoneMsg)
	if done.action != "restart" || done.err != nil {
		t.Errorf("result = %+v, want restart ok", done)
	}
	if got := calls(mock); !slices.Equal(got, []string{"Stop db", "Start db"}) {
		t.Errorf("calls = %v, want [Stop db Start db]", got)
	}
}

func TestMenuRestartSkipsStartWhenStopFails(t *testing.T) {
	app, mock := testApp()
	mock.ActionFn = func(_ context.Context, action, _ string) error {
		if action == "Stop" {
			return errors.New("boom")
		}
		return nil
	}
	m := newContainerMenu(app, runningContainer())
	_, cmd := m.update(key('r'))
	if done := cmd().(actionDoneMsg); done.err == nil {
		t.Error("restart must report the failed stop")
	}
	if got := calls(mock); !slices.Equal(got, []string{"Stop db"}) {
		t.Errorf("calls = %v, want only [Stop db]", got)
	}
}

func TestMenuStopRemoveConfirmsThenStopsAndDeletes(t *testing.T) {
	app, mock := testApp()
	m := newContainerMenu(app, runningContainer())
	next, _ := m.update(key('X'))
	c, ok := next.(*confirmScreen)
	if !ok {
		t.Fatalf("X = %T, want *confirmScreen", next)
	}
	if !strings.Contains(c.question, "stop and delete container") {
		t.Errorf("question = %q, want stop and delete", c.question)
	}
	_, cmd := c.update(key('y'))
	if done := cmd().(actionDoneMsg); done.action != "remove" || done.err != nil {
		t.Errorf("result = %+v, want remove ok", done)
	}
	if got := calls(mock); !slices.Equal(got, []string{"Stop db", "Delete db"}) {
		t.Errorf("calls = %v, want [Stop db Delete db]", got)
	}
}

func TestMenuRemoveStoppedDeletesOnly(t *testing.T) {
	app, mock := testApp()
	m := newContainerMenu(app, stoppedContainer())
	next, _ := m.update(key('R'))
	c, ok := next.(*confirmScreen)
	if !ok {
		t.Fatalf("R = %T, want *confirmScreen", next)
	}
	_, cmd := c.update(key('y'))
	cmd()
	if got := calls(mock); !slices.Equal(got, []string{"Delete db"}) {
		t.Errorf("calls = %v, want [Delete db]", got)
	}
}

func TestMenuHotkeyForOtherStateIgnored(t *testing.T) {
	app, mock := testApp()
	m := newContainerMenu(app, stoppedContainer())
	for _, r := range []rune{'K', 'X', 'e', 'w', 'r', 'Z'} {
		next, cmd := m.update(key(r))
		if next != screen(m) || cmd != nil {
			t.Errorf("%c on stopped menu = (%T, %v), want menu unchanged", r, next, cmd)
		}
	}
	if len(mock.CallList()) != 0 {
		t.Errorf("no client calls expected, got %v", calls(mock))
	}
}

func TestMenuEnterOnStopReturnsConfirm(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	for m.items[m.cursor].label != "stop" {
		m.update(key('j'))
	}
	next, _ := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if _, ok := next.(*confirmScreen); !ok {
		t.Errorf("enter on stop = %T, want *confirmScreen", next)
	}
}

func TestMenuCancelCloses(t *testing.T) {
	app, _ := testApp()
	for _, r := range []rune{'c'} {
		m := newContainerMenu(app, runningContainer())
		next, cmd := m.update(key(r))
		if next != nil || cmd != nil {
			t.Errorf("%c = (%T, %v), want (nil, nil)", r, next, cmd)
		}
	}
}

func TestMenuEscCloses(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	next, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if next != nil || cmd != nil {
		t.Errorf("esc = (%T, %v), want (nil, nil)", next, cmd)
	}
}

func TestMenuViewShowsHotkeys(t *testing.T) {
	app, _ := testApp()
	m := newContainerMenu(app, runningContainer())
	out := m.view(80, 24)
	for _, want := range menuEntries(m) {
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
			if !strings.Contains(line, "single view") {
				t.Errorf("marker on %q, want the single view line", line)
			}
		}
	}
	if marker != 1 {
		t.Errorf("marker count = %d, want 1", marker)
	}
}

func TestEnterOnGridOpensMenu(t *testing.T) {
	app, _ := testApp()
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID:     "db",
		Status: &cli.Status{State: "running"},
	}}})
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := app.top().(*menuScreen)
	for m.items[m.cursor].label != "stop" {
		app.Update(key('j'))
	}
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if _, ok := app.top().(*confirmScreen); !ok {
		t.Fatalf("expected confirm screen, got %T", app.top())
	}
	_, cmd := app.Update(key('y'))
	if cmd == nil {
		t.Fatal("expected action command from confirm")
	}
	cmd()
	found := false
	for _, c := range mock.CallList() {
		if c.Method == "Stop" && c.ID == "db" {
			found = true
		}
	}
	if !found {
		t.Errorf("Stop not called via menu flow: %+v", mock.CallList())
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
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	app.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if _, ok := app.top().(*gridScreen); !ok {
		t.Fatalf("expected grid after esc, got %T", app.top())
	}
	if len(mock.CallList()) != 0 {
		t.Errorf("no client calls expected, got %+v", mock.CallList())
	}
}

func TestVolumeMenuDeleteFlow(t *testing.T) {
	app, mock := testApp()
	app.Update(key('2'))
	app.Update(volumesResultMsg{volumes: []cli.Volume{{
		ID:            "vol1",
		Configuration: cli.VolumeConfig{Name: "vol1"},
	}}})
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, ok := app.top().(*menuScreen)
	if !ok {
		t.Fatalf("expected menu on volumes tab, got %T", app.top())
	}
	if m.title != "vol1" {
		t.Errorf("menu title = %q, want vol1", m.title)
	}
	if got := menuEntries(m); !slices.Equal(got, []string{"[R] remove", "[c] cancel"}) {
		t.Fatalf("volume menu = %v, want [[R] remove [c] cancel]", got)
	}
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // Delete -> confirm
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
	for _, call := range mock.CallList() {
		if call.Method == "VolumeDelete" && call.ID == "vol1" {
			found = true
		}
	}
	if !found {
		t.Errorf("VolumeDelete not called via menu: %+v", mock.CallList())
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
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	app.Update(key('j')) // cursor to Cancel
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if _, ok := app.top().(*volumesScreen); !ok {
		t.Fatalf("expected volumes tab after Cancel, got %T", app.top())
	}
	if len(mock.CallList()) != 0 {
		t.Errorf("no client calls expected, got %+v", mock.CallList())
	}
}

func TestImageMenuDeleteFlow(t *testing.T) {
	app, mock := testApp()
	app.Update(key('3'))
	app.Update(imagesResultMsg{images: []cli.Image{{
		ID:            "img1",
		Configuration: cli.ImageConfig{Name: "docker.io/library/postgres:17"},
	}}})
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, ok := app.top().(*menuScreen)
	if !ok {
		t.Fatalf("expected menu on images tab, got %T", app.top())
	}
	if m.title != "docker.io/library/postgres:17" {
		t.Errorf("menu title = %q, want image ref", m.title)
	}
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // Delete -> confirm
	if _, ok := app.top().(*confirmScreen); !ok {
		t.Fatalf("expected confirm screen, got %T", app.top())
	}
	_, cmd := app.Update(key('y'))
	if cmd == nil {
		t.Fatal("expected delete command")
	}
	cmd()
	found := false
	for _, call := range mock.CallList() {
		if call.Method == "ImageDelete" && call.ID == "docker.io/library/postgres:17" {
			found = true
		}
	}
	if !found {
		t.Errorf("ImageDelete not called via menu: %+v", mock.CallList())
	}
}
