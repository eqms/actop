package ui

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/config"
	"github.com/eqms/actop/metrics"
	"github.com/eqms/actop/model"
	"github.com/eqms/actop/update"
)

const (
	backoffStart = 2 * time.Second
	backoffMax   = 30 * time.Second
)

// App is the Bubble Tea root model: it owns the poll loop, the container
// store and the screen stack.
type App struct {
	client   cli.Client
	cfg      config.Config
	store    *model.Store
	engine   *metrics.Engine
	hostCPUs int
	version  string

	width, height int
	stack         []screen
	tabs          []screen
	activeTab     int

	volumes []cli.Volume
	images  []cli.Image
	sysInfo *cli.SystemStatus
	updates update.Result
	// checker is nil in production (the zero-value Checker is the real one);
	// tests inject one pointing at an httptest server.
	checker *update.Checker
	columns []string

	sortField     model.SortField
	sortReversed  bool
	allContainers bool
	// hideHeader is ctop's H: the version/tab/info line goes, the
	// connection banner and status messages stay. Session only.
	hideHeader bool

	connOK    bool
	lastOK    time.Time
	backoff   time.Duration
	sysDetail string
	statusMsg string
}

func NewApp(client cli.Client, cfg config.Config, version string) *App {
	app := &App{
		client:        client,
		cfg:           cfg,
		store:         model.NewStore(),
		engine:        metrics.NewEngine(),
		hostCPUs:      runtime.NumCPU(),
		version:       version,
		sortField:     model.SortField(cfg.SortField),
		sortReversed:  cfg.SortReversed,
		allContainers: cfg.AllContainers,
		connOK:        true,
	}
	app.columns = resolveColumns(cfg.Columns)
	app.tabs = []screen{newGridScreen(app), newVolumesScreen(app), newImagesScreen(app), newSystemScreen(app)}
	app.stack = []screen{app.tabs[0]}
	return app
}

// resolveColumns validates the configured column keys against the registry
// and forces "name" to stay first; empty config means all columns.
func resolveColumns(configured []string) []string {
	if len(configured) == 0 {
		out := make([]string, len(allColumnKeys))
		copy(out, allColumnKeys)
		return out
	}
	known := map[string]bool{}
	for _, k := range allColumnKeys {
		known[k] = true
	}
	out := []string{"name"}
	for _, k := range configured {
		if k != "name" && known[k] {
			out = append(out, k)
		}
	}
	return out
}

// tabNames labels the base views in the header, index-aligned with tabs.
var tabNames = []string{"containers", "volumes", "images", "system"}

// switchTab replaces the base screen and fetches the new tab's data.
func (a *App) switchTab(idx int) tea.Cmd {
	if idx < 0 || idx >= len(a.tabs) || idx == a.activeTab {
		return nil
	}
	a.activeTab = idx
	a.stack[0] = a.tabs[idx]
	return a.tabFetchCmd()
}

// tabFetchCmd returns the fetch command for the active tab's data source.
func (a *App) tabFetchCmd() tea.Cmd {
	switch a.activeTab {
	case 1:
		return fetchVolumesCmd(a.client)
	case 2:
		return fetchImagesCmd(a.client)
	case 3:
		// system tab aggregates everything
		return tea.Batch(fetchSystemStatusCmd(a.client), fetchVolumesCmd(a.client), fetchImagesCmd(a.client))
	default:
		return nil
	}
}

// inputCapturer lets a base screen veto tab-switch keys while it is
// consuming raw text input (e.g. the grid's filter field).
type inputCapturer interface{ capturesInput() bool }

func (a *App) baseCapturesInput() bool {
	if c, ok := a.stack[0].(inputCapturer); ok {
		return c.capturesInput()
	}
	return false
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		fetchListCmd(a.client, a.allContainers),
		fetchStatsCmd(a.client),
		tickCmd(a.interval()),
		checkUpdatesCmd(a.client, a.cfg, a.version, a.checker),
	)
}

// interval is the poll cadence — the config value, or the current backoff
// while the API server is unreachable.
func (a *App) interval() time.Duration {
	if !a.connOK && a.backoff > 0 {
		return a.backoff
	}
	return a.cfg.PollInterval()
}

// push adds a screen on top of the stack. Logs screens start streaming.
func (a *App) push(s screen) tea.Cmd {
	a.stack = append(a.stack, s)
	if l, ok := s.(*logsScreen); ok {
		return l.start()
	}
	return nil
}

func (a *App) top() screen { return a.stack[len(a.stack)-1] }

// retryNow resets the backoff and polls immediately.
func (a *App) retryNow() tea.Cmd {
	a.backoff = 0
	return tea.Batch(fetchListCmd(a.client, a.allContainers), fetchStatsCmd(a.client), a.tabFetchCmd())
}

func (a *App) handlePollError(err error) tea.Cmd {
	if errors.Is(err, cli.ErrAPIServerDown) {
		wasOK := a.connOK
		a.connOK = false
		if a.backoff == 0 {
			a.backoff = backoffStart
		} else if a.backoff < backoffMax {
			a.backoff *= 2
			if a.backoff > backoffMax {
				a.backoff = backoffMax
			}
		}
		if wasOK {
			return fetchSystemStatusCmd(a.client)
		}
		return nil
	}
	a.statusMsg = err.Error()
	return nil
}

func (a *App) markConnected() {
	a.connOK = true
	a.backoff = 0
	a.lastOK = time.Now()
	a.sysDetail = ""
}

// routeToTop delivers msg to the current top screen. The index is captured
// first because the screen may push new screens during update; nil means
// the screen wants to be popped.
func (a *App) routeToTop(msg tea.Msg) tea.Cmd {
	idx := len(a.stack) - 1
	next, cmd := a.stack[idx].update(msg)
	if next == nil {
		if idx > 0 {
			a.stack = append(a.stack[:idx], a.stack[idx+1:]...)
		}
	} else {
		a.stack[idx] = next
	}
	return cmd
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		return a, a.routeToTop(msg)

	case tea.KeyPressMsg:
		if m.String() == "ctrl+c" {
			a.closeAll()
			return a, tea.Quit
		}
		// tab switching only applies to the base view, never to overlays
		// or while the base screen consumes text input
		if len(a.stack) == 1 && !a.baseCapturesInput() {
			switch m.String() {
			case "tab":
				return a, a.switchTab((a.activeTab + 1) % len(a.tabs))
			case "1", "2", "3", "4":
				return a, a.switchTab(int(m.String()[0] - '1'))
			case "H":
				a.hideHeader = !a.hideHeader
				return a, nil
			}
		}
		return a, a.routeToTop(msg)

	case tickMsg:
		return a, tea.Batch(
			fetchListCmd(a.client, a.allContainers),
			fetchStatsCmd(a.client),
			a.tabFetchCmd(),
			tickCmd(a.interval()),
		)

	case listResultMsg:
		if m.err != nil {
			return a, a.handlePollError(m.err)
		}
		a.markConnected()
		a.store.ApplyList(m.items)
		return a, nil

	case statsResultMsg:
		if m.err != nil {
			return a, a.handlePollError(m.err)
		}
		a.markConnected()
		a.store.ApplyStats(m.entries, m.ts, a.engine, a.hostCPUs)
		return a, nil

	case volumesResultMsg:
		if m.err != nil {
			return a, a.handlePollError(m.err)
		}
		a.markConnected()
		a.volumes = m.volumes
		return a, nil

	case imagesResultMsg:
		if m.err != nil {
			return a, a.handlePollError(m.err)
		}
		a.markConnected()
		a.images = m.images
		return a, nil

	case systemStatusMsg:
		if m.err != nil {
			a.sysDetail = m.err.Error()
		} else if m.status != nil {
			a.sysInfo = m.status
			a.sysDetail = "system status: " + m.status.Status
		}
		return a, nil

	case updatesMsg:
		a.updates = m.result
		return a, nil

	case actionDoneMsg:
		if m.err != nil {
			a.statusMsg = fmt.Sprintf("%s %s: %s", m.action, m.id, m.err)
		} else {
			a.statusMsg = fmt.Sprintf("%s %s: ok", m.action, m.id)
		}
		return a, a.retryNow()

	case execFinishedMsg:
		if m.err != nil {
			a.statusMsg = "exec: " + m.err.Error()
		}
		return a, nil
	}

	// everything else (mouse, log lines, ...) goes to the top screen
	return a, a.routeToTop(msg)
}

// saveSettings is ctop's S: the grid settings of this session go into
// config.toml; poll interval, shell and update check keep their loaded
// values.
func (a *App) saveSettings() {
	cfg := a.cfg
	cfg.SortField = string(a.sortField)
	cfg.SortReversed = a.sortReversed
	cfg.AllContainers = a.allContainers
	cfg.Columns = append([]string(nil), a.columns...)
	path, err := config.Save(cfg)
	if err != nil {
		a.statusMsg = "save config: " + err.Error()
		return
	}
	a.cfg = cfg
	a.statusMsg = "wrote config to " + path
}

// closeAll releases resources of stacked screens (log subprocesses).
func (a *App) closeAll() {
	for _, s := range a.stack {
		if l, ok := s.(*logsScreen); ok {
			l.close()
		}
	}
}

func (a *App) header() string {
	leftPlain := fmt.Sprintf("actop v%s", a.version)
	left := styleHeader.Render(leftPlain)
	var tabs, tabsPlain strings.Builder
	tabs.WriteString("  ")
	tabsPlain.WriteString("  ")
	for i, name := range tabNames {
		label := fmt.Sprintf("[%d]%s", i+1, name)
		if i == a.activeTab {
			tabs.WriteString(styleTitle.Render(label))
		} else {
			tabs.WriteString(styleDim.Render(label))
		}
		tabs.WriteString(" ")
		tabsPlain.WriteString(label + " ")
	}
	info := fmt.Sprintf(" %d containers · sort %s · poll %s",
		a.store.Len(), a.sortField, a.interval().Truncate(time.Millisecond))
	line := left + tabs.String() + styleDim.Render(info)

	if !a.connOK {
		banner := fmt.Sprintf("API server not reachable — retrying every %s (r = retry now)", a.interval())
		if a.sysDetail != "" {
			banner += " · " + safe(a.sysDetail)
		}
		stale := ""
		if !a.lastOK.IsZero() {
			stale = styleDim.Render(fmt.Sprintf("  showing data from %s", a.lastOK.Format("15:04:05")))
		}
		return a.joinHeader(line, styleErr.Render(banner)+stale)
	}

	// The badge is the least important thing on this line: it yields to the
	// error banner above and to a terminal too narrow for the tab bar.
	if badge := a.updateBadge(); badge != "" {
		plain := leftPlain + tabsPlain.String() + info
		if a.width == 0 || displayWidth(plain)+displayWidth(badge) <= a.width {
			line += styleWarn.Render(badge)
		}
	}

	if a.statusMsg != "" {
		// carries container IDs and CLI stderr; sanitized here rather than at
		// every assignment so no future call site can bypass it
		return a.joinHeader(line, styleWarn.Render(safe(a.statusMsg)))
	}
	return a.joinHeader(line, "")
}

// joinHeader puts the info line above an optional banner or status line.
// With the header hidden only the latter remains: it reports a lost
// connection or the outcome of an action, which H must not swallow.
func (a *App) joinHeader(line, extra string) string {
	switch {
	case a.hideHeader:
		return extra
	case extra == "":
		return line
	default:
		return line + "\n" + extra
	}
}

// updateBadge is the compact header hint; the system tab carries the detail.
func (a *App) updateBadge() string {
	switch n := a.updates.Count(); n {
	case 0:
		return ""
	case 1:
		return "  ⬆ 1 update"
	default:
		return fmt.Sprintf("  ⬆ %d updates", n)
	}
}

func (a *App) View() tea.View {
	// v2 moved the terminal modes from program options onto the view
	v := tea.NewView(a.Body())
	v.AltScreen = true
	return v
}

// Body renders the frame's text. Kept separate from View so tests can assert
// on the content without unwrapping a tea.View.
func (a *App) Body() string {
	head := a.header()
	if head == "" {
		return a.top().view(a.width, a.height-1)
	}
	body := a.top().view(a.width, a.height-strings.Count(head, "\n")-2)
	return head + "\n" + body
}
