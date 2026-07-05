# Menu v2 (ctop-faithful, all tabs) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ctop-faithful action menu — fixed item list with disabled (dimmed) entries, plain label list, Cancel entry — and the same enter-menu on the volumes and images tabs.

**Architecture:** `menuScreen` becomes generic (title + items; actions capture their target in closures). `menuItem` gets a `disabled` flag replacing the nil-build separator. Three constructors: `newContainerMenu`, `newVolumeMenu`, `newImageMenu`. Volumes/images extract their delete-confirm construction into shared helpers used by both the `D` hotkey and the menu.

**Tech Stack:** Go 1.26, Bubble Tea v1, Lipgloss v1 (existing dependencies only).

**Spec:** `docs/superpowers/specs/2026-07-05-actop-menu-v2-design.md`

## Global Constraints

- Container menu: fixed order Details, Logs, Shell, Open URL, Start, Stop, Kill, Delete, Cancel — always all entries; disabled = dimmed + skipped by cursor, never hidden.
- Enabled rules: Shell/Stop/Kill only running; Open URL only running AND `WebURL() != ""`; Start only NOT running; Details/Logs/Delete/Cancel always.
- Menu keys: `j/k/down/up` (skip disabled, clamp), `enter` runs, `esc`/`q` closes. NO per-item hotkeys inside the menu.
- Direct hotkeys on grid/volumes/images tabs stay unchanged (`l`, `e`, `o`, `S`, `X`, `K`, `D`).
- Rendering: centered `styleMenuPanel`, title line, one label per row, `▸ ` + styleSelected marker, disabled rows styleDim. No hotkey column, no separator, no footer.
- Destructive actions always route through the existing confirm screens (container `confirmAction`; volume confirm keeps the "(data is lost)" warning).
- Version 0.5.0, bilingual RELEASE_NOTES, README keys table + "Neu in v0.5" + roadmap heading retitled to v0.6.
- English identifiers/comments; commit prefixes `[ADD]`/`[CHG]`/`[FIX]`; every task ends with `go test ./ui/` green and `make lint` → `0 issues.` before commit.

---

### Task 1: Generic menuScreen with fixed container list

**Files:**
- Modify: `ui/menu.go` (full rewrite, see Step 3)
- Modify: `ui/grid.go:97-100` (`enter` case: `newMenuScreen(g.app, c)` → `newContainerMenu(g.app, c)`)
- Test: `ui/menu_test.go` (full rewrite, see Step 1 — the old state-filtered/hotkey/separator tests are obsolete by design)

**Interfaces:**
- Consumes: `newDetailScreen(app, id)`, `newLogsScreen(app, id)` + `(*logsScreen).start()`, `execShellCmd(client, cfg, id)`, `openURLCmd(url)`, `confirmAction(app, action, id)` — all existing.
- Produces (Task 2 relies on these): `menuItem{label string; disabled bool; build func(*menuScreen) (screen, tea.Cmd)}`; `menuScreen{app *App; title string; items []menuItem; cursor int}`; `newMenuScreen(app *App, title string, items []menuItem) *menuScreen` (generic, cursor on first enabled item); `cancelItem() menuItem`; `newContainerMenu(app *App, c *model.Container) *menuScreen`.

- [ ] **Step 1: Rewrite the test file (failing tests)**

Replace the entire content of `ui/menu_test.go` with:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ui/ -run 'TestContainerMenu|TestMenu|TestEnterOnGrid' -v 2>&1 | head -20`
Expected: compile FAIL — `undefined: newContainerMenu`, `m.title undefined`, `it.disabled undefined`

- [ ] **Step 3: Rewrite ui/menu.go**

Replace the entire content of `ui/menu.go` with:

```go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/eqms/actop/model"
)

// menuItem is one entry in an action menu. Disabled entries stay visible
// but dimmed (ctop keeps the full list so the layout never shifts) and are
// skipped by the cursor.
type menuItem struct {
	label    string
	disabled bool
	build    func(m *menuScreen) (screen, tea.Cmd)
}

// menuScreen is the ctop-style action menu, generic over its target: the
// items capture what they act on in closures at open time.
type menuScreen struct {
	app    *App
	title  string
	items  []menuItem
	cursor int
}

// newMenuScreen places the cursor on the first enabled item.
func newMenuScreen(app *App, title string, items []menuItem) *menuScreen {
	m := &menuScreen{app: app, title: title, items: items}
	for i, it := range items {
		if !it.disabled {
			m.cursor = i
			break
		}
	}
	return m
}

// cancelItem closes the menu; ctop lists it as the last entry.
func cancelItem() menuItem {
	return menuItem{label: "Cancel", build: func(m *menuScreen) (screen, tea.Cmd) {
		return nil, nil
	}}
}

// newContainerMenu builds the fixed container action list. Entries that do
// not apply to the container's current state are disabled, not hidden.
func newContainerMenu(app *App, c *model.Container) *menuScreen {
	id := c.ID
	url := c.WebURL()
	running := c.Running()
	items := []menuItem{
		{label: "Details", build: func(m *menuScreen) (screen, tea.Cmd) {
			return newDetailScreen(m.app, id), nil
		}},
		{label: "Logs", build: func(m *menuScreen) (screen, tea.Cmd) {
			// starts streaming itself: menu selection replaces the menu via
			// routeToTop instead of App.push (which normally calls start)
			l := newLogsScreen(m.app, id)
			return l, l.start()
		}},
		{label: "Shell", disabled: !running, build: func(m *menuScreen) (screen, tea.Cmd) {
			return nil, execShellCmd(m.app.client, m.app.cfg, id)
		}},
		{label: "Open URL", disabled: !running || url == "", build: func(m *menuScreen) (screen, tea.Cmd) {
			return nil, openURLCmd(url)
		}},
		{label: "Start", disabled: running, build: confirmItem("start", id)},
		{label: "Stop", disabled: !running, build: confirmItem("stop", id)},
		{label: "Kill", disabled: !running, build: confirmItem("kill", id)},
		{label: "Delete", build: confirmItem("delete", id)},
		cancelItem(),
	}
	return newMenuScreen(app, c.ID, items)
}

func confirmItem(action, id string) func(*menuScreen) (screen, tea.Cmd) {
	return func(m *menuScreen) (screen, tea.Cmd) {
		return confirmAction(m.app, action, id), nil
	}
}

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
		// cursor can only rest on enabled items; guard stays for safety
		if it := m.items[m.cursor]; !it.disabled {
			return it.build(m)
		}
	}
	return m, nil
}

// move shifts the cursor by delta, skipping disabled entries and clamping
// at both ends.
func (m *menuScreen) move(delta int) {
	i := m.cursor + delta
	for i >= 0 && i < len(m.items) && m.items[i].disabled {
		i += delta
	}
	if i >= 0 && i < len(m.items) {
		m.cursor = i
	}
}

// menuWidth is the label column width inside the panel.
const menuWidth = 16

func (m *menuScreen) view(width, height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(m.title) + "\n\n")
	for i, it := range m.items {
		label := pad(it.label, menuWidth)
		switch {
		case i == m.cursor:
			b.WriteString("▸ " + styleSelected.Render(label) + "\n")
		case it.disabled:
			b.WriteString("  " + styleDim.Render(label) + "\n")
		default:
			b.WriteString("  " + label + "\n")
		}
	}
	panel := styleMenuPanel.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}
```

In `ui/grid.go`, change the `enter` case:

```go
	case "enter":
		if c := g.selected(); c != nil {
			return g, g.app.push(newContainerMenu(g.app, c))
		}
```

- [ ] **Step 4: Run the full ui suite**

Run: `go test ./ui/ -v 2>&1 | tail -15`
Expected: PASS — all rewritten menu tests plus all pre-existing tests.

- [ ] **Step 5: Lint and commit**

```bash
make lint
git add ui/menu.go ui/menu_test.go ui/grid.go
git commit -m "[CHG] menu: ctop-faithful fixed list with disabled entries, Cancel item"
```

---

### Task 2: Volume and image menus

**Files:**
- Modify: `ui/volumes.go:61-67` (extract confirm helper, add `enter` case)
- Modify: `ui/images.go:61-67` (same)
- Modify: `ui/menu.go` (append two constructors)
- Test: `ui/menu_test.go` (append)

**Interfaces:**
- Consumes from Task 1: `menuItem`, `menuScreen`, `newMenuScreen(app, title, items)`, `cancelItem()`.
- Consumes existing: `newConfirmScreen(question, cmd)`, `volumeDeleteCmd(client, name)`, `imageDeleteCmd(client, ref)`.
- Produces: `newVolumeMenu(app *App, name string) *menuScreen`, `newImageMenu(app *App, ref string) *menuScreen`, `volumeDeleteConfirm(app *App, name string) *confirmScreen`, `imageDeleteConfirm(app *App, ref string) *confirmScreen`.

- [ ] **Step 1: Write the failing tests**

Append to `ui/menu_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ui/ -run 'TestVolumeMenu|TestImageMenu' -v 2>&1 | head -12`
Expected: FAIL — `expected menu on volumes tab, got *ui.volumesScreen` (enter not wired yet)

- [ ] **Step 3: Implement**

Append to `ui/menu.go`:

```go
// newVolumeMenu and newImageMenu give the other tabs the same enter-menu.
func newVolumeMenu(app *App, name string) *menuScreen {
	items := []menuItem{
		{label: "Delete", build: func(m *menuScreen) (screen, tea.Cmd) {
			return volumeDeleteConfirm(m.app, name), nil
		}},
		cancelItem(),
	}
	return newMenuScreen(app, name, items)
}

func newImageMenu(app *App, ref string) *menuScreen {
	items := []menuItem{
		{label: "Delete", build: func(m *menuScreen) (screen, tea.Cmd) {
			return imageDeleteConfirm(m.app, ref), nil
		}},
		cancelItem(),
	}
	return newMenuScreen(app, ref, items)
}
```

In `ui/volumes.go`, replace the `D` case and add an `enter` case; add the shared helper:

```go
	case "enter":
		if vol := v.selected(); vol != nil {
			return v, v.app.push(newVolumeMenu(v.app, vol.Configuration.Name))
		}
	case "D":
		if vol := v.selected(); vol != nil {
			return v, v.app.push(volumeDeleteConfirm(v.app, vol.Configuration.Name))
		}
```

```go
// volumeDeleteConfirm is shared by the D hotkey and the enter menu.
func volumeDeleteConfirm(app *App, name string) *confirmScreen {
	q := fmt.Sprintf("delete volume %q? (data is lost)", name)
	return newConfirmScreen(q, volumeDeleteCmd(app.client, name))
}
```

In `ui/images.go`, the same pattern:

```go
	case "enter":
		if img := s.selected(); img != nil {
			return s, s.app.push(newImageMenu(s.app, img.Configuration.Name))
		}
	case "D":
		if img := s.selected(); img != nil {
			return s, s.app.push(imageDeleteConfirm(s.app, img.Configuration.Name))
		}
```

```go
// imageDeleteConfirm is shared by the D hotkey and the enter menu.
func imageDeleteConfirm(app *App, ref string) *confirmScreen {
	q := fmt.Sprintf("delete image %q?", ref)
	return newConfirmScreen(q, imageDeleteCmd(app.client, ref))
}
```

- [ ] **Step 4: Run the full ui suite**

Run: `go test ./ui/ -v 2>&1 | tail -12`
Expected: PASS — including the pre-existing `TestVolumeDeleteConfirm` (D-hotkey path, now via the shared helper).

- [ ] **Step 5: Lint and commit**

```bash
make lint
git add ui/menu.go ui/menu_test.go ui/volumes.go ui/images.go
git commit -m "[ADD] volumes/images tabs: enter action menu (Delete, Cancel)"
```

---

### Task 3: Help, README, release chores

**Files:**
- Modify: `ui/help.go:23` (enter entry)
- Modify: `README.md` (keys table DE+EN, "Neu in v0.4" → new v0.5 section, roadmap heading)
- Modify: `VERSION`, `RELEASE_NOTES.md`

**Interfaces:**
- Consumes: shipped behavior from Tasks 1-2.

- [ ] **Step 1: Update help entry**

In `ui/help.go`, replace the `enter` entry:

```go
	{"enter", "action menu (containers / volumes / images)"},
```

- [ ] **Step 2: Update README keys table (both languages)**

German (`| enter |` row): replace with

```markdown
| `enter` | Aktionsmenü (ctop-Stil, feste Liste) — auch in Volumes/Images |
```

English row:

```markdown
| `enter` | Action menu (ctop-style, fixed list) — also on volumes/images |
```

- [ ] **Step 3: Add "Neu in v0.5" sections and retitle roadmap**

In the German section, insert ABOVE `### Neu in v0.4`:

```markdown
### Neu in v0.5

- **ctop-treues Aktionsmenü**: feste Einträge (Details, Logs, Shell, Open URL, Start, Stop, Kill, Delete, Cancel) — nicht anwendbare ausgegraut statt ausgeblendet, schlichte Liste ohne Hotkey-Spalte, expliziter Cancel-Eintrag
- **Menüs in allen Tabs**: `enter` öffnet auch bei Volumes und Images ein Aktionsmenü (Delete mit Rückfrage, Cancel)

```

Retitle `### Roadmap (v0.5)` → `### Roadmap (v0.6)` (items unchanged).

In the English section, insert ABOVE `### New in v0.4`:

```markdown
### New in v0.5

- **ctop-faithful action menu**: fixed entries (Details, Logs, Shell, Open URL, Start, Stop, Kill, Delete, Cancel) — inapplicable ones dimmed instead of hidden, plain list without hotkey column, explicit Cancel entry
- **Menus on all tabs**: `enter` also opens an action menu on volumes and images (delete with confirmation, Cancel)

```

Retitle `### Roadmap (v0.5)` → `### Roadmap (v0.6)`.

- [ ] **Step 4: VERSION and RELEASE_NOTES**

`VERSION`: replace content with

```
0.5.0
```

`RELEASE_NOTES.md` — insert directly under `## Deutsche Release Notes`:

```markdown
### v0.5.0 (2026-07-05)

#### Geändert
- **Aktionsmenü im ctop-Stil überarbeitet**: feste Eintragsliste in konstanter Reihenfolge (Details, Logs, Shell, Open URL, Start, Stop, Kill, Delete, Cancel); nicht anwendbare Einträge werden ausgegraut und vom Cursor übersprungen statt ausgeblendet — kein Springen der Liste mehr. Schlichte Optik ohne Hotkey-Spalte, Separator und Fußzeile; expliziter Cancel-Eintrag. Direkte Hotkeys im Grid bleiben unverändert.

#### Neue Features
- **Aktionsmenü in allen Tabs**: `enter` öffnet jetzt auch bei Volumes und Images ein Menü (Delete mit Rückfrage — bei Volumes inkl. Datenverlust-Warnung — und Cancel)

---
```

And directly under `## English Release Notes`:

```markdown
### v0.5.0 (2026-07-05)

#### Changed
- **Action menu reworked in ctop style**: fixed entry list in constant order (Details, Logs, Shell, Open URL, Start, Stop, Kill, Delete, Cancel); inapplicable entries are dimmed and skipped by the cursor instead of hidden — the list no longer shifts. Plain look without hotkey column, separator and footer; explicit Cancel entry. Direct grid hotkeys remain unchanged.

#### Added
- **Action menu on all tabs**: `enter` now also opens a menu on volumes and images (delete with confirmation — including the data-loss warning for volumes — and Cancel)

---
```

- [ ] **Step 5: Full verification**

```bash
go build ./... && go test ./... && make lint
grep -n "brew install eqms/tap/actop" README.md
```

Expected: build OK, all PASS, `0 issues.`; the Homebrew install instructions are untouched (2 matches).

- [ ] **Step 6: Commit**

```bash
git add ui/help.go README.md VERSION RELEASE_NOTES.md
git commit -m "[ADD] v0.5.0: ctop-faithful menu, menus on all tabs"
```
