package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/config"
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

// The grid keys follow ctop's grid.go: o and → open the single view, ← the
// logs, w the browser, S saves the settings, H toggles the header.

func TestGridOOpensSingleView(t *testing.T) {
	app, _ := appWith("running")
	app.Update(key('o'))
	if _, ok := app.top().(*detailScreen); !ok {
		t.Errorf("o pushed %T, want *detailScreen", app.top())
	}
}

func TestGridRightOpensSingleView(t *testing.T) {
	app, _ := appWith("stopped")
	app.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if _, ok := app.top().(*detailScreen); !ok {
		t.Errorf("→ pushed %T, want *detailScreen", app.top())
	}
}

func TestGridLeftOpensLogs(t *testing.T) {
	app, _ := appWith("running")
	app.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	l, ok := app.top().(*logsScreen)
	if !ok {
		t.Fatalf("← pushed %T, want *logsScreen", app.top())
	}
	l.close()
}

func TestGridWOpensBrowserOnlyWithURL(t *testing.T) {
	app, _ := appWith("running")
	// No port and no IP: nothing to open, and the user is told so. A command
	// is never run here -- it would launch open(1).
	if _, cmd := app.Update(key('w')); cmd != nil {
		t.Error("w without a URL must not return a command")
	}
	if !strings.Contains(app.statusMsg, "no published port") {
		t.Errorf("statusMsg = %q, want a hint", app.statusMsg)
	}
}

func TestGridSSavesSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	app, mock := appWith("stopped")
	app.sortField = model.SortCPU
	app.sortReversed = true
	app.allContainers = false
	app.Update(key('S'))

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SortField != "cpu" || !cfg.SortReversed || cfg.AllContainers {
		t.Errorf("saved = %+v, want cpu, reversed, stopped hidden", cfg)
	}
	if !strings.Contains(app.statusMsg, "wrote config to") {
		t.Errorf("statusMsg = %q, want the path written", app.statusMsg)
	}
	if len(mock.CallList()) != 0 {
		t.Errorf("S must no longer start a container: %v", calls(mock))
	}
}

// ctop's D dumps state into its debug log, which actop does not have; the
// key is simply free now. Deleting happens through the enter menu.
func TestGridDDoesNothing(t *testing.T) {
	app, mock := appWith("running")
	_, cmd := app.Update(key('D'))
	if _, ok := app.top().(*gridScreen); !ok || cmd != nil {
		t.Errorf("D = (%T, cmd %v), want grid unchanged", app.top(), cmd)
	}
	if len(mock.CallList()) != 0 {
		t.Errorf("no client calls expected, got %v", calls(mock))
	}
}

func TestHTogglesHeader(t *testing.T) {
	app, _ := appWith("running")
	app.Update(key('H'))
	if strings.Contains(app.Body(), "actop v") {
		t.Error("header still shown after H")
	}
	app.Update(key('H'))
	if !strings.Contains(app.Body(), "actop v") {
		t.Error("header not back after second H")
	}
}

// The connection banner is not decoration: it says the data is stale.
func TestHiddenHeaderKeepsConnectionBanner(t *testing.T) {
	app, _ := appWith("running")
	app.Update(key('H'))
	app.Update(listResultMsg{err: cli.ErrAPIServerDown})
	if !strings.Contains(app.Body(), "API server not reachable") {
		t.Error("banner hidden together with the header")
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
