package ui

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/config"
	"github.com/eqms/actop/update"
)

// key builds the press event for a printable character. In Bubble Tea v2 the
// rune lives in Code and the literal text in Text, and it is String() that the
// screens switch on.
func key(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func testApp() (*App, *cli.MockClient) {
	mock := &cli.MockClient{}
	return NewApp(mock, config.Default(), "test"), mock
}

func TestListResultPopulatesStore(t *testing.T) {
	app, _ := testApp()
	items := []cli.ListItem{{
		ID: "db",
		Configuration: cli.Configuration{
			Image: cli.ImageRef{Reference: "postgres:16"},
		},
		Status: &cli.Status{State: "running"},
	}}
	app.Update(listResultMsg{items: items})
	if app.store.Len() != 1 {
		t.Fatalf("store not populated: %d", app.store.Len())
	}
	if !app.connOK {
		t.Error("successful poll must mark connection OK")
	}
}

func TestAPIServerDownSetsBackoff(t *testing.T) {
	app, _ := testApp()
	app.Update(listResultMsg{err: cli.ErrAPIServerDown})
	if app.connOK {
		t.Fatal("connOK must be false after ErrAPIServerDown")
	}
	if app.backoff != backoffStart {
		t.Errorf("backoff = %s, want %s", app.backoff, backoffStart)
	}
	// repeated failures double up to the cap
	for i := 0; i < 10; i++ {
		app.Update(statsResultMsg{err: cli.ErrAPIServerDown})
	}
	if app.backoff != backoffMax {
		t.Errorf("backoff = %s, want cap %s", app.backoff, backoffMax)
	}
	// successful poll resets
	app.Update(listResultMsg{})
	if !app.connOK || app.backoff != 0 {
		t.Error("successful poll must reset connection state")
	}
}

func TestConfirmYesTriggersAction(t *testing.T) {
	app, mock := testApp()
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID:     "db",
		Status: &cli.Status{State: "running"},
	}}})

	// X on the grid pushes the stop confirmation
	app.Update(key('X'))
	if _, ok := app.top().(*confirmScreen); !ok {
		t.Fatalf("expected confirm screen on top, got %T", app.top())
	}

	// y answers the confirmation; the returned command runs the action
	_, cmd := app.Update(key('y'))
	if cmd == nil {
		t.Fatal("expected action command from confirm")
	}
	msg := cmd()
	done, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("expected actionDoneMsg, got %T", msg)
	}
	if done.action != "stop" || done.id != "db" || done.err != nil {
		t.Errorf("unexpected action result: %+v", done)
	}

	found := false
	for _, c := range mock.CallList() {
		if c.Method == "Stop" && c.ID == "db" {
			found = true
		}
	}
	if !found {
		t.Errorf("Stop not called on client: %+v", mock.CallList())
	}
	if _, ok := app.top().(*gridScreen); !ok {
		t.Errorf("confirm screen not popped, top is %T", app.top())
	}
}

func TestConfirmNoDoesNothing(t *testing.T) {
	app, mock := testApp()
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID:     "db",
		Status: &cli.Status{State: "running"},
	}}})
	app.Update(key('X'))
	app.Update(key('n'))
	for _, c := range mock.CallList() {
		if c.Method == "Stop" {
			t.Error("Stop must not be called after 'n'")
		}
	}
	if _, ok := app.top().(*gridScreen); !ok {
		t.Errorf("confirm screen not popped, top is %T", app.top())
	}
}

func TestTabSwitching(t *testing.T) {
	app, _ := testApp()
	if _, ok := app.top().(*gridScreen); !ok {
		t.Fatalf("initial tab must be the container grid, got %T", app.top())
	}

	// "2" switches to volumes and fetches them
	_, cmd := app.Update(key('2'))
	if _, ok := app.top().(*volumesScreen); !ok {
		t.Fatalf("expected volumes tab, got %T", app.top())
	}
	if cmd == nil {
		t.Error("tab switch must trigger a fetch command")
	}

	// tab cycles volumes -> images
	app.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if _, ok := app.top().(*imagesScreen); !ok {
		t.Fatalf("expected images tab after cycle, got %T", app.top())
	}

	// volumes result lands in app state
	app.Update(volumesResultMsg{volumes: []cli.Volume{{ID: "v1"}}})
	if len(app.volumes) != 1 {
		t.Errorf("volumes not stored: %d", len(app.volumes))
	}
}

func TestTabKeysIgnoredWhileFiltering(t *testing.T) {
	app, _ := testApp()
	app.Update(key('f')) // grid enters filter mode
	app.Update(key('2')) // must go into the filter, not switch tabs
	if _, ok := app.top().(*gridScreen); !ok {
		t.Fatalf("tab switched during filter input, top is %T", app.top())
	}
	if got := app.top().(*gridScreen).filter.Value(); got != "2" {
		t.Errorf("filter value = %q, want \"2\"", got)
	}
}

func TestVolumeDeleteConfirm(t *testing.T) {
	app, mock := testApp()
	app.Update(key('2'))
	app.Update(volumesResultMsg{volumes: []cli.Volume{{
		ID:            "vol1",
		Configuration: cli.VolumeConfig{Name: "vol1"},
	}}})
	app.Update(key('D'))
	if _, ok := app.top().(*confirmScreen); !ok {
		t.Fatalf("expected confirm screen, got %T", app.top())
	}
	_, cmd := app.Update(key('y'))
	if cmd == nil {
		t.Fatal("expected delete command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("delete command returned nil msg")
	}
	found := false
	for _, c := range mock.CallList() {
		if c.Method == "VolumeDelete" && c.ID == "vol1" {
			found = true
		}
	}
	if !found {
		t.Errorf("VolumeDelete not called: %+v", mock.CallList())
	}
}

func TestResolveColumns(t *testing.T) {
	all := resolveColumns(nil)
	if len(all) != len(allColumnKeys) {
		t.Errorf("empty config must enable all columns, got %v", all)
	}
	custom := resolveColumns([]string{"cpu", "mem", "bogus"})
	want := []string{"name", "cpu", "mem"}
	if len(custom) != 3 || custom[0] != want[0] || custom[1] != want[1] || custom[2] != want[2] {
		t.Errorf("resolveColumns = %v, want %v (name forced, bogus dropped)", custom, want)
	}
}

// The space bar reaches the screen as "space", not " ". Driving it through
// App.Update rather than calling toggle directly is the point: this is the
// wiring that silently broke in the v1 -> v2 key API change.
func TestColumnsToggleViaSpaceKey(t *testing.T) {
	app, _ := testApp()
	app.Update(key('c'))
	cs, ok := app.top().(*columnsScreen)
	if !ok {
		t.Fatalf("expected columns screen, got %T", app.top())
	}
	cs.cursor = 1 // "state"
	app.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if cs.enabled("state") {
		t.Error("space must toggle the column under the cursor")
	}
	app.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if !cs.enabled("state") {
		t.Error("space must toggle back on")
	}
}

func TestColumnsToggle(t *testing.T) {
	app, _ := testApp()
	app.Update(key('c'))
	cs, ok := app.top().(*columnsScreen)
	if !ok {
		t.Fatalf("expected columns screen, got %T", app.top())
	}
	cs.toggle("state")
	if cs.enabled("state") {
		t.Error("state should be disabled after toggle")
	}
	cs.toggle("state")
	if !cs.enabled("state") {
		t.Error("state should be re-enabled in registry order")
	}
	cs.toggle("name")
	if !cs.enabled("name") {
		t.Error("name must be locked on")
	}
}

func TestSystemTab(t *testing.T) {
	app, _ := testApp()
	_, cmd := app.Update(key('4'))
	if _, ok := app.top().(*systemScreen); !ok {
		t.Fatalf("expected system tab, got %T", app.top())
	}
	if cmd == nil {
		t.Error("system tab must fetch status/volumes/images")
	}
	app.Update(systemStatusMsg{status: &cli.SystemStatus{Status: "running", APIServerVersion: "v1"}})
	if app.sysInfo == nil || app.sysInfo.Status != "running" {
		t.Errorf("system status not stored: %+v", app.sysInfo)
	}
}

func TestViewRendersAllTabs(t *testing.T) {
	app, _ := testApp()
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID:     "db",
		Status: &cli.Status{State: "running"},
	}}})
	for i := 0; i < 4; i++ {
		app.switchTab(i)
		out := app.Body()
		if out == "" {
			t.Errorf("tab %d renders empty view", i)
		}
	}
}

func TestTickSchedulesPoll(t *testing.T) {
	app, _ := testApp()
	_, cmd := app.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("tick must schedule commands")
	}
}

func TestUpdatesMsgStoresResult(t *testing.T) {
	app, _ := testApp()
	app.Update(updatesMsg{result: update.Result{
		Components: []update.Component{
			{Name: "container", Current: "1.1.0", Latest: "1.2.0", Hint: "/usr/local/bin/update-container.sh"},
		},
	}})
	if app.updates.Count() != 1 {
		t.Fatalf("updates.Count = %d, want 1", app.updates.Count())
	}
}

// Opting out must prevent the command from existing at all -- not merely
// from rendering its result.
func TestCheckUpdatesCmdRespectsOptOut(t *testing.T) {
	t.Setenv(config.EnvNoUpdateCheck, "1")
	mock := &cli.MockClient{}
	if cmd := checkUpdatesCmd(mock, config.Default(), "0.6.1", nil); cmd != nil {
		t.Fatal("opt-out must yield a nil command")
	}
	if len(mock.CallList()) != 0 {
		t.Errorf("opt-out must not touch the client, got %+v", mock.CallList())
	}
}

// The command must ask the CLI for the installed container version and hand
// both versions to the checker.
func TestCheckUpdatesCmdReportsBothComponents(t *testing.T) {
	t.Setenv(config.EnvNoUpdateCheck, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/apple/container/releases/latest":
			_, _ = fmt.Fprint(w, `{"tag_name":"1.2.0"}`)
		case "/repos/eqms/actop/releases/latest":
			_, _ = fmt.Fprint(w, `{"tag_name":"v0.7.0"}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	mock := &cli.MockClient{VersionValue: "1.1.0"}
	ck := &update.Checker{BaseURL: srv.URL, CacheDir: t.TempDir()}
	cmd := checkUpdatesCmd(mock, config.Default(), "0.6.1", ck)
	if cmd == nil {
		t.Fatal("want a command")
	}
	msg, ok := cmd().(updatesMsg)
	if !ok {
		t.Fatalf("command returned %T, want updatesMsg", cmd())
	}
	if msg.result.Count() != 2 {
		t.Errorf("Count = %d, want 2: %+v", msg.result.Count(), msg.result.Components)
	}
	var sawVersion bool
	for _, c := range mock.CallList() {
		if c.Method == "Version" {
			sawVersion = true
		}
	}
	if !sawVersion {
		t.Error("the command must query the container CLI version")
	}
}

// A container CLI that is missing or broken must not suppress the actop
// check -- the two components are independent.
func TestCheckUpdatesCmdSurvivesVersionError(t *testing.T) {
	t.Setenv(config.EnvNoUpdateCheck, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/eqms/actop/releases/latest" {
			_, _ = fmt.Fprint(w, `{"tag_name":"0.7.0"}`)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	mock := &cli.MockClient{VersionErr: errors.New("no such binary")}
	ck := &update.Checker{BaseURL: srv.URL, CacheDir: t.TempDir()}
	msg := checkUpdatesCmd(mock, config.Default(), "0.6.1", ck)().(updatesMsg)
	if msg.result.Count() != 1 || msg.result.Components[0].Name != "actop" {
		t.Errorf("want only actop reported, got %+v", msg.result.Components)
	}
}

func withUpdates(app *App, n int) {
	comps := make([]update.Component, 0, n)
	names := []string{"container", "actop"}
	for i := 0; i < n; i++ {
		comps = append(comps, update.Component{
			Name: names[i%len(names)], Current: "1.1.0", Latest: "1.2.0", Hint: "brew upgrade actop",
		})
	}
	app.Update(updatesMsg{result: update.Result{Components: comps}})
}

func TestHeaderShowsUpdateBadge(t *testing.T) {
	app, _ := testApp()
	app.width, app.height = 200, 40
	if strings.Contains(app.header(), "update") {
		t.Fatal("no result yet, but the header already advertises updates")
	}
	withUpdates(app, 2)
	if !strings.Contains(app.header(), "2 updates") {
		t.Errorf("header lacks the badge:\n%s", app.header())
	}
	app.updates = update.Result{}
	withUpdates(app, 1)
	if !strings.Contains(app.header(), "1 update") || strings.Contains(app.header(), "1 updates") {
		t.Errorf("singular badge wrong:\n%s", app.header())
	}
}

// The error banner owns the header while the API server is unreachable; an
// update hint must not compete with it.
func TestHeaderHidesBadgeWhileDisconnected(t *testing.T) {
	app, _ := testApp()
	app.width, app.height = 200, 40
	withUpdates(app, 2)
	app.connOK = false
	if strings.Contains(app.header(), "2 updates") {
		t.Errorf("badge must be suppressed while disconnected:\n%s", app.header())
	}
}

// On a narrow terminal the tab bar matters more than the hint.
func TestHeaderDropsBadgeWhenTooNarrow(t *testing.T) {
	app, _ := testApp()
	app.width, app.height = 40, 40
	withUpdates(app, 2)
	if strings.Contains(app.header(), "2 updates") {
		t.Errorf("badge must not push the tab bar out of a 40-column terminal:\n%s", app.header())
	}
}

func TestSystemTabRendersUpdates(t *testing.T) {
	app, _ := testApp()
	app.width, app.height = 200, 40
	app.Update(updatesMsg{result: update.Result{Components: []update.Component{
		{Name: "container", Current: "1.1.0", Latest: "1.2.0", Hint: "/usr/local/bin/update-container.sh"},
		{Name: "actop", Current: "0.6.1", Latest: "0.7.0", Hint: "brew upgrade actop"},
	}}})
	view := newSystemScreen(app).view(200, 40)
	for _, want := range []string{
		"UPDATES",
		"container", "1.1.0 → 1.2.0 available", "/usr/local/bin/update-container.sh",
		"actop", "0.6.1 → 0.7.0 available", "brew upgrade actop",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("system tab is missing %q:\n%s", want, view)
		}
	}
}

// Nothing outdated means the section is absent entirely, not an empty header.
func TestSystemTabOmitsUpdatesWhenNone(t *testing.T) {
	app, _ := testApp()
	view := newSystemScreen(app).view(200, 40)
	if strings.Contains(view, "UPDATES") {
		t.Errorf("empty result must not render a section:\n%s", view)
	}
}
