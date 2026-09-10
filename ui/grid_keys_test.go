package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/model"
)

func appWith(state string) (*App, *cli.MockClient) {
	app, mock := testApp()
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID:     "db",
		Status: &cli.Status{State: state},
	}}})
	return app, mock
}

func TestGridStartRunsWithoutConfirm(t *testing.T) {
	app, mock := appWith("stopped")
	_, cmd := app.Update(key('S'))
	if _, ok := app.top().(*gridScreen); !ok {
		t.Fatalf("S pushed %T, want no dialog", app.top())
	}
	if cmd == nil {
		t.Fatal("S returned no command")
	}
	cmd()
	if got := calls(mock); !slices.Equal(got, []string{"Start db"}) {
		t.Errorf("calls = %v, want [Start db]", got)
	}
}

// Apple Container refuses to delete a running container without --force.
// D on a running one therefore stops it first, and says so in the prompt.
func TestGridDeleteRunningStopsFirst(t *testing.T) {
	app, mock := appWith("running")
	app.Update(key('D'))
	c, ok := app.top().(*confirmScreen)
	if !ok {
		t.Fatalf("D pushed %T, want confirm", app.top())
	}
	if !strings.Contains(c.question, "stop and delete container") {
		t.Errorf("question = %q, want stop and delete", c.question)
	}
	_, cmd := app.Update(key('y'))
	cmd()
	if got := calls(mock); !slices.Equal(got, []string{"Stop db", "Delete db"}) {
		t.Errorf("calls = %v, want [Stop db Delete db]", got)
	}
}

func TestGridDeleteStoppedDeletesOnly(t *testing.T) {
	app, mock := appWith("stopped")
	app.Update(key('D'))
	c, ok := app.top().(*confirmScreen)
	if !ok {
		t.Fatalf("D pushed %T, want confirm", app.top())
	}
	if !strings.Contains(c.question, "delete container") || strings.Contains(c.question, "stop") {
		t.Errorf("question = %q, want a plain delete", c.question)
	}
	_, cmd := app.Update(key('y'))
	cmd()
	if got := calls(mock); !slices.Equal(got, []string{"Delete db"}) {
		t.Errorf("calls = %v, want [Delete db]", got)
	}
}

func TestGridSortOpensSelection(t *testing.T) {
	app, _ := appWith("running")
	app.sortField = model.SortCPU
	app.Update(key('s'))
	m, ok := app.top().(*menuScreen)
	if !ok {
		t.Fatalf("s pushed %T, want sort menu", app.top())
	}
	if got := menuLabels(m); !slices.Equal(got, sortFieldLabels()) {
		t.Errorf("sort menu = %v, want every sort field %v", got, sortFieldLabels())
	}
	if m.items[m.cursor].label != string(model.SortCPU) {
		t.Errorf("cursor on %q, want the active field cpu", m.items[m.cursor].label)
	}
	app.Update(key('j'))
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if app.sortField != model.SortMem {
		t.Errorf("sortField = %s, want mem", app.sortField)
	}
	if _, ok := app.top().(*gridScreen); !ok {
		t.Errorf("sort menu not closed, top is %T", app.top())
	}
}

func TestGridSortEscKeepsField(t *testing.T) {
	app, _ := appWith("running")
	app.sortField = model.SortCPU
	app.Update(key('s'))
	app.Update(key('j'))
	app.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if app.sortField != model.SortCPU {
		t.Errorf("esc changed sortField to %s", app.sortField)
	}
}

func sortFieldLabels() []string {
	var out []string
	for _, f := range model.SortFields {
		out = append(out, string(f))
	}
	return out
}
