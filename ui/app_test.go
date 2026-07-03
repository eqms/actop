package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/config"
)

func key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
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
	for _, c := range mock.Calls {
		if c.Method == "Stop" && c.ID == "db" {
			found = true
		}
	}
	if !found {
		t.Errorf("Stop not called on client: %+v", mock.Calls)
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
	for _, c := range mock.Calls {
		if c.Method == "Stop" {
			t.Error("Stop must not be called after 'n'")
		}
	}
	if _, ok := app.top().(*gridScreen); !ok {
		t.Errorf("confirm screen not popped, top is %T", app.top())
	}
}

func TestTickSchedulesPoll(t *testing.T) {
	app, _ := testApp()
	_, cmd := app.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("tick must schedule commands")
	}
}
