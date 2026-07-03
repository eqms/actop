package ui

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/config"
	"github.com/eqms/actop/metrics"
	"github.com/eqms/actop/model"
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

	sortField     model.SortField
	sortReversed  bool
	allContainers bool

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
	app.stack = []screen{newGridScreen(app)}
	return app
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		fetchListCmd(a.client, a.allContainers),
		fetchStatsCmd(a.client),
		tickCmd(a.interval()),
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
	return tea.Batch(fetchListCmd(a.client, a.allContainers), fetchStatsCmd(a.client))
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

	case tea.KeyMsg:
		if m.String() == "ctrl+c" {
			a.closeAll()
			return a, tea.Quit
		}
		return a, a.routeToTop(msg)

	case tickMsg:
		return a, tea.Batch(
			fetchListCmd(a.client, a.allContainers),
			fetchStatsCmd(a.client),
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

	case systemStatusMsg:
		if m.err != nil {
			a.sysDetail = m.err.Error()
		} else if m.status != nil {
			a.sysDetail = "system status: " + m.status.Status
		}
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

// closeAll releases resources of stacked screens (log subprocesses).
func (a *App) closeAll() {
	for _, s := range a.stack {
		if l, ok := s.(*logsScreen); ok {
			l.close()
		}
	}
}

func (a *App) header() string {
	left := styleHeader.Render(fmt.Sprintf("actop v%s", a.version))
	info := fmt.Sprintf("  %d containers · sort %s · poll %s",
		a.store.Len(), a.sortField, a.interval().Truncate(time.Millisecond))
	line := left + styleDim.Render(info)

	if !a.connOK {
		banner := fmt.Sprintf("API server not reachable — retrying every %s (r = retry now)", a.interval())
		if a.sysDetail != "" {
			banner += " · " + a.sysDetail
		}
		stale := ""
		if !a.lastOK.IsZero() {
			stale = styleDim.Render(fmt.Sprintf("  showing data from %s", a.lastOK.Format("15:04:05")))
		}
		return line + "\n" + styleErr.Render(banner) + stale
	}
	if a.statusMsg != "" {
		return line + "\n" + styleWarn.Render(a.statusMsg)
	}
	return line
}

func (a *App) View() string {
	head := a.header()
	body := a.top().view(a.width, a.height-strings.Count(head, "\n")-2)
	return head + "\n" + body
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
