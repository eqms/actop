# Container Action Menu (ctop-style) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pressing `enter` on a container in the containers tab opens a centered, state-filtered action menu (ctop-style); destructive actions keep the confirm dialog, grid hotkeys stay unchanged.

**Architecture:** New `menuScreen` in the existing screen stack (`ui/screen.go` interface: `update` returns replacement screen, nil pops). The menu is built from a container snapshot at open time and holds only the container ID plus a closure per item. Selecting an item replaces the menu with the follow-up screen (detail/logs/confirm) or pops the menu and runs a command (shell/open URL).

**Tech Stack:** Go 1.26, Bubble Tea v1, Lipgloss v1 (all existing dependencies; nothing new).

**Spec:** `docs/superpowers/specs/2026-07-04-actop-menu-design.md`

## Global Constraints

- No new dependencies; menu = new screen in the existing stack pattern (confirm/columns/help).
- Containers tab only; volumes/images keep current behavior.
- Cursor starts on "Details" so `enter enter` reaches the detail view.
- Stop/Kill/Delete/Start go through the existing `confirmScreen` — never execute directly.
- Direct grid hotkeys (`l`, `e`, `o`, `S`, `X`, `K`, `D`) stay unchanged.
- All code comments and identifiers in English; commit prefixes `[ADD]`/`[CHG]`/`[FIX]`.
- Target version 0.4.0, bilingual release notes entry.
- Every task ends with `go test ./ui/` green; run `make lint` before each commit.

---

### Task 1: menuScreen construction (state-filtered items)

**Files:**
- Create: `ui/menu.go`
- Test: `ui/menu_test.go` (new file; helpers `testApp()`/`key()` come from `ui/app_test.go`, same package)

**Interfaces:**
- Consumes: `*App` (ui/app.go), `*model.Container` with `Running() bool`, `WebURL() string`, `ID string` (model/container.go), `newDetailScreen(app, id)` (ui/detail.go:22), `newLogsScreen(app, id)` + `(*logsScreen).start()` (ui/logs.go), `execShellCmd(client, cfg, id)`, `openURLCmd(url)` (ui/commands.go), `confirmAction(app, action, id)` (ui/grid.go:294).
- Produces: `newMenuScreen(app *App, c *model.Container) *menuScreen`; type `menuScreen{app *App; id string; items []menuItem; cursor int}`; type `menuItem{label, hotkey string; build func(*menuScreen) (screen, tea.Cmd)}` — a nil `build` marks the separator. Task 2 adds `update`, Task 3 adds `view`.

- [ ] **Step 1: Write the failing tests**

Create `ui/menu_test.go`:

```go
package ui

import (
	"testing"

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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ui/ -run 'TestMenu' -v`
Expected: FAIL — `undefined: menuScreen`, `undefined: newMenuScreen`

- [ ] **Step 3: Write the implementation**

Create `ui/menu.go`:

```go
package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/eqms/actop/model"
)

// menuItem is one selectable action in the container menu.
// A nil build func marks the separator between navigation and
// destructive actions.
type menuItem struct {
	label  string
	hotkey string
	build  func(m *menuScreen) (screen, tea.Cmd)
}

// menuScreen is the ctop-style per-container action menu. It captures
// only the container ID; the item list is a state snapshot from open time.
type menuScreen struct {
	app    *App
	id     string
	items  []menuItem
	cursor int
}

// newMenuScreen builds the state-filtered item list. Running containers
// get the full action set, stopped ones only what applies to them.
func newMenuScreen(app *App, c *model.Container) *menuScreen {
	m := &menuScreen{app: app, id: c.ID}
	m.items = []menuItem{
		{label: "Details", hotkey: "enter", build: buildDetail},
		{label: "Logs", hotkey: "l", build: buildLogs},
	}
	if c.Running() {
		m.items = append(m.items, menuItem{label: "Shell", hotkey: "e", build: buildShell})
		if url := c.WebURL(); url != "" {
			m.items = append(m.items, menuItem{label: "Open URL", hotkey: "o",
				build: func(m *menuScreen) (screen, tea.Cmd) { return nil, openURLCmd(url) }})
		}
		m.items = append(m.items,
			menuItem{}, // separator
			menuItem{label: "Stop", hotkey: "X", build: buildConfirm("stop")},
			menuItem{label: "Kill", hotkey: "K", build: buildConfirm("kill")},
			menuItem{label: "Delete", hotkey: "D", build: buildConfirm("delete")},
		)
	} else {
		m.items = append(m.items,
			menuItem{}, // separator
			menuItem{label: "Start", hotkey: "S", build: buildConfirm("start")},
			menuItem{label: "Delete", hotkey: "D", build: buildConfirm("delete")},
		)
	}
	return m
}

func buildDetail(m *menuScreen) (screen, tea.Cmd) {
	return newDetailScreen(m.app, m.id), nil
}

// buildLogs starts streaming itself because the screen replaces the menu
// instead of going through App.push (which normally calls start).
func buildLogs(m *menuScreen) (screen, tea.Cmd) {
	l := newLogsScreen(m.app, m.id)
	return l, l.start()
}

func buildShell(m *menuScreen) (screen, tea.Cmd) {
	return nil, execShellCmd(m.app.client, m.app.cfg, m.id)
}

func buildConfirm(action string) func(*menuScreen) (screen, tea.Cmd) {
	return func(m *menuScreen) (screen, tea.Cmd) {
		return confirmAction(m.app, action, m.id), nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./ui/ -run 'TestMenu' -v`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
make lint
git add ui/menu.go ui/menu_test.go
git commit -m "[ADD] menuScreen: state-filtered container action items"
```

---

### Task 2: Menu key handling (navigate, select, hotkeys, close)

**Files:**
- Modify: `ui/menu.go` (append)
- Test: `ui/menu_test.go` (append)

**Interfaces:**
- Consumes: `menuScreen`/`menuItem` from Task 1; `screen` interface (ui/screen.go: `update(msg tea.Msg) (screen, tea.Cmd)`, nil return pops).
- Produces: `(*menuScreen).update(msg tea.Msg) (screen, tea.Cmd)` and `(*menuScreen).move(delta int)`. `esc`/`q` → `(nil, nil)`; `enter` → run selected item; item hotkey → run that item; `j/k/down/up` → move cursor, skipping the separator, clamping at both ends.

- [ ] **Step 1: Write the failing tests**

Append to `ui/menu_test.go`:

```go
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
```

Add `tea "github.com/charmbracelet/bubbletea"` to the test file imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ui/ -run 'TestMenu' -v`
Expected: FAIL — `m.update undefined`

- [ ] **Step 3: Write the implementation**

Append to `ui/menu.go`:

```go
func (m *menuScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "q":
		return nil, nil
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "enter":
		return m.items[m.cursor].build(m)
	default:
		for _, it := range m.items {
			if it.build != nil && it.hotkey == key.String() {
				return it.build(m)
			}
		}
	}
	return m, nil
}

// move shifts the cursor by delta, skipping separators and clamping
// at both ends.
func (m *menuScreen) move(delta int) {
	i := m.cursor + delta
	for i >= 0 && i < len(m.items) && m.items[i].build == nil {
		i += delta
	}
	if i >= 0 && i < len(m.items) {
		m.cursor = i
	}
}
```

Also append a minimal placeholder so `*menuScreen` satisfies the `screen`
interface (the `update` return type requires it at compile time); Task 3
replaces this with the real implementation:

```go
// view is completed in the rendering task; minimal placeholder so
// menuScreen satisfies the screen interface.
func (m *menuScreen) view(width, height int) string { return "" }
```

Note: the "Details" item's displayed hotkey is `enter`, which is already
handled by the `enter` case — the hotkey loop never sees it. That is
intentional (enter = run selection).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./ui/ -run 'TestMenu' -v`
Expected: PASS (9 tests)

- [ ] **Step 5: Commit**

```bash
make lint
git add ui/menu.go ui/menu_test.go
git commit -m "[ADD] menuScreen key handling: navigation, hotkeys, esc"
```

---

### Task 3: Menu rendering (centered bordered panel)

**Files:**
- Modify: `ui/menu.go` (append), `ui/styles.go`
- Test: `ui/menu_test.go` (append)

**Interfaces:**
- Consumes: `pad(s string, w int)` (ui/grid.go:285), `styleTitle`, `styleSelected`, `styleDim` (ui/styles.go), lipgloss (existing dependency).
- Produces: `(*menuScreen).view(width, height int) string` — completes the `screen` interface; `styleMenuPanel` in ui/styles.go.

- [ ] **Step 1: Write the failing test**

Append to `ui/menu_test.go` (add `"strings"` to the test file imports):

```go
func TestMenuViewRendersItemsAndTitle(t *testing.T) {
	app, _ := testApp()
	m := newMenuScreen(app, &model.Container{ID: "db", State: "running"})
	out := m.view(80, 24)
	for _, want := range []string{"db", "Details", "Logs", "Shell", "Stop", "Kill", "Delete"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ui/ -run 'TestMenuView' -v`
Expected: FAIL — `m.view undefined`

- [ ] **Step 3: Write the implementation**

Add to `ui/styles.go` (inside the existing `var (...)` block):

```go
	styleMenuPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("6")).
			Padding(0, 2)
```

In `ui/menu.go`, replace Task 2's placeholder `view` with the real
implementation (add `"strings"` and `"github.com/charmbracelet/lipgloss"`
to imports):

```go
// menuWidth is the inner label column width; hotkeys render right of it.
const menuWidth = 16

func (m *menuScreen) view(width, height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(m.id) + "\n\n")
	for i, it := range m.items {
		if it.build == nil {
			b.WriteString("  " + styleDim.Render(strings.Repeat("─", menuWidth+6)) + "\n")
			continue
		}
		label := pad(it.label, menuWidth)
		if i == m.cursor {
			b.WriteString("▸ " + styleSelected.Render(label) + " " + styleDim.Render(it.hotkey) + "\n")
		} else {
			b.WriteString("  " + label + " " + styleDim.Render(it.hotkey) + "\n")
		}
	}
	b.WriteString("\n" + styleDim.Render("↑/↓ select · enter · esc"))
	panel := styleMenuPanel.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./ui/ -v -run 'TestMenu'`
Expected: PASS (10 tests)

- [ ] **Step 5: Commit**

```bash
make lint
git add ui/menu.go ui/menu_test.go ui/styles.go
git commit -m "[ADD] menuScreen rendering: centered bordered panel"
```

---

### Task 4: Grid wiring, help entry, integration tests

**Files:**
- Modify: `ui/grid.go:97-100` (enter case), `ui/help.go:23` (help entry)
- Test: `ui/menu_test.go` (append)

**Interfaces:**
- Consumes: `newMenuScreen` (Task 1), completed `screen` implementation (Tasks 2-3), `(*App).push` (ui/app.go:151), `(*gridScreen).selected()` (ui/grid.go:33).
- Produces: `enter` on the grid pushes the menu; end-to-end flow menu → confirm → mock client call.

- [ ] **Step 1: Write the failing integration tests**

Append to `ui/menu_test.go`:

```go
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
	if m.id != "db" {
		t.Errorf("menu id = %q, want db", m.id)
	}
}

func TestMenuStopFlowCallsClient(t *testing.T) {
	app, mock := testApp()
	app.Update(listResultMsg{items: []cli.ListItem{{
		ID:     "db",
		Status: &cli.Status{State: "running"},
	}}})
	app.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open menu
	app.Update(key('X'))                       // menu hotkey -> confirm
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
```

Note: `TestMenuStopFlowCallsClient` exercises the confirm-screen replacement
path in `routeToTop` (menu is replaced by confirm via `a.stack[idx] = next`,
confirm pops with nil) — no direct-execution shortcut.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ui/ -run 'TestEnterOnGrid|TestMenuStopFlow|TestMenuEscReturns' -v`
Expected: FAIL — top screen is `*detailScreen` (grid still opens detail on enter)

- [ ] **Step 3: Wire the grid and help**

In `ui/grid.go`, replace the `enter` case (lines 97-100):

```go
	case "enter":
		if c := g.selected(); c != nil {
			return g, g.app.push(newMenuScreen(g.app, c))
		}
```

In `ui/help.go`, replace the `enter` entry (line 23):

```go
	{"enter", "container action menu (details, logs, shell, ...)"},
```

- [ ] **Step 4: Run the full ui test suite**

Run: `go test ./ui/ -v`
Expected: PASS — including all pre-existing tests (none reference the old
enter→detail behavior; `TestConfirmYesTriggersAction` uses the direct `X`
hotkey, which is unchanged)

- [ ] **Step 5: Commit**

```bash
make lint
git add ui/grid.go ui/help.go ui/menu_test.go
git commit -m "[CHG] grid: enter opens container action menu (ctop-style)"
```

---

### Task 5: Release chores (version, release notes, full verification)

**Files:**
- Modify: `VERSION`, `RELEASE_NOTES.md`

**Interfaces:**
- Consumes: everything from Tasks 1-4.
- Produces: shippable v0.4.0.

- [ ] **Step 1: Bump version**

`VERSION`: replace content with

```
0.4.0
```

- [ ] **Step 2: Add release notes**

In `RELEASE_NOTES.md`, insert directly under `## Deutsche Release Notes`:

```markdown
### v0.4.0 (2026-07-04)

#### Neue Features
- **Container-Aktionsmenü** (`enter`, ctop-Stil): zentriertes Panel mit zustandsabhängigen Aktionen — laufend: Details, Logs, Shell, Open URL (falls Port/IP), Stop, Kill, Delete; gestoppt: Details, Logs, Start, Delete. Navigation mit `j/k/↑/↓`, Auswahl per `enter` oder Aktions-Hotkey, `esc` schließt. Destruktive Aktionen behalten den Bestätigungsdialog; die direkten Grid-Hotkeys bleiben erhalten.

---
```

And directly under `## English Release Notes`:

```markdown
### v0.4.0 (2026-07-04)

#### Added
- **Container action menu** (`enter`, ctop-style): centered panel with state-filtered actions — running: Details, Logs, Shell, Open URL (if port/IP), Stop, Kill, Delete; stopped: Details, Logs, Start, Delete. Navigate with `j/k/↑/↓`, select via `enter` or the action hotkey, `esc` closes. Destructive actions keep the confirmation dialog; direct grid hotkeys remain available.

---
```

- [ ] **Step 3: Full verification**

```bash
go build ./... && go test ./... && make lint
```

Expected: build OK, all packages PASS, `0 issues.`

Manual smoke test (real binary, running container required):
`make run-dev` or `go run .` — press `enter` on a container: menu opens
centered, `j/k` skips the separator, `esc` returns, `X` → confirm → `n`
cancels. Verify `enter enter` opens the detail view.

- [ ] **Step 4: Commit**

```bash
git add VERSION RELEASE_NOTES.md
git commit -m "[ADD] v0.4.0: ctop-style container action menu"
```
