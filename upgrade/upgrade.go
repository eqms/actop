// Package upgrade updates Apple Container in place: it stops the container
// service, runs Apple's own updater, starts the service again and brings
// back the containers that were running before, then reports what happened.
//
// Apple's update-container.sh refuses to run while the service is up, and
// stopping the service stops every container with it. Doing those steps by
// hand is what this package replaces.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/redact"
	"github.com/eqms/actop/update"
)

const (
	readTimeout  = 10 * time.Second
	stopTimeout  = 60 * time.Second
	startTimeout = 120 * time.Second
)

// ErrAborted means the user declined before anything was changed.
var ErrAborted = errors.New("aborted -- nothing was changed")

// Outcome is what became of one container that was running before.
type Outcome int

const (
	Restarted   Outcome = iota
	StartFailed         // it still exists but would not start
	Gone                // deleted with the service, i.e. it ran with --rm
)

type ContainerResult struct {
	ID      string
	Outcome Outcome
	Err     error
}

// Report is the outcome of a run that got past the point of no return --
// the service stop -- or found nothing to do.
type Report struct {
	Before, After, Latest string
	UpToDate              bool
	Service               string // `container system status` afterwards
	ScriptErr             error  // Apple's updater failed
	ServiceErr            error  // the service did not come back
	Containers            []ContainerResult
}

// OK is false when anything needs the user's attention. A --rm container
// that is gone is expected and does not count.
func (r Report) OK() bool {
	if r.ScriptErr != nil || r.ServiceErr != nil {
		return false
	}
	for _, c := range r.Containers {
		if c.Outcome == StartFailed {
			return false
		}
	}
	return true
}

// Runner holds the collaborators; main wires the real ones, tests fakes.
type Runner struct {
	Client cli.Client
	// Latest returns the newest Apple Container release, uncached.
	Latest func(ctx context.Context) (string, bool)
	// Script is the path to Apple's updater.
	Script string
	// RunScript runs the updater attached to the terminal, so sudo can ask
	// for the password.
	RunScript func(ctx context.Context, path string) error
	// Confirm asks a yes/no question; only called when containers run.
	Confirm func(question string) bool
	Out     io.Writer
}

func (r *Runner) step(format string, args ...any) {
	_, _ = fmt.Fprintf(r.Out, "→ "+format+"\n", args...)
}

// Run returns an error only for failures before the service is stopped;
// from there on it carries on as far as it can -- a failed update must not
// leave the machine without its containers -- and puts every failure into
// the report.
func (r *Runner) Run(ctx context.Context) (Report, error) {
	rctx, cancel := context.WithTimeout(ctx, readTimeout)
	before, err := r.Client.Version(rctx)
	cancel()
	if err != nil {
		return Report{}, fmt.Errorf("cannot read the installed container version: %w", err)
	}
	cur, ok := update.Parse(before)
	if !ok {
		return Report{}, fmt.Errorf("unrecognised container version %q", redact.Line(before))
	}

	latest, ok := r.Latest(ctx)
	if !ok {
		return Report{}, errors.New("could not look up the latest Apple Container release on GitHub -- nothing was stopped")
	}
	rep := Report{Before: cur.String(), Latest: latest}
	if remote, ok := update.Parse(latest); !ok || !cur.Less(remote) {
		rep.UpToDate = true
		return rep, nil
	}

	if _, err := os.Stat(r.Script); err != nil {
		return Report{}, fmt.Errorf("the updater from Apple's package, %s, is missing (%w); install the new release from https://github.com/apple/container/releases", r.Script, err)
	}

	running := r.runningIDs(ctx)
	_, _ = fmt.Fprintf(r.Out, "Apple Container %s → %s\n", rep.Before, latest)
	if len(running) > 0 {
		q := fmt.Sprintf("Stopping the container service also stops %d running container(s): %s. Continue?",
			len(running), redact.Line(strings.Join(running, ", ")))
		if !r.Confirm(q) {
			return Report{}, ErrAborted
		}
	}

	r.step("stopping the container service")
	sctx, cancel := context.WithTimeout(ctx, stopTimeout)
	err = r.Client.SystemStop(sctx)
	cancel()
	if err != nil {
		return Report{}, fmt.Errorf("container system stop failed, update not attempted: %w", err)
	}

	r.step("running %s", r.Script)
	rep.ScriptErr = r.RunScript(ctx, r.Script)

	r.step("starting the container service")
	sctx, cancel = context.WithTimeout(ctx, startTimeout)
	rep.ServiceErr = r.Client.SystemStart(sctx)
	cancel()
	if rep.ServiceErr == nil {
		rep.Containers = r.restart(ctx, running)
	}

	rctx, cancel = context.WithTimeout(ctx, readTimeout)
	defer cancel()
	if after, err := r.Client.Version(rctx); err == nil {
		rep.After = after
	}
	if st, err := r.Client.SystemStatus(rctx); err == nil {
		rep.Service = st.Status
	} else {
		rep.Service = "not running (" + err.Error() + ")"
	}
	return rep, nil
}

// runningIDs lists the running containers. A service that is already down
// has none, which is not an error here.
func (r *Runner) runningIDs(ctx context.Context) []string {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	items, err := r.Client.List(ctx, false)
	if err != nil {
		return nil
	}
	var ids []string
	for _, it := range items {
		if it.Status != nil && it.Status.State == "running" {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

// restart starts the containers that were running before. Those that no
// longer exist were deleted with the service; if the list itself fails,
// every start is attempted and reports for itself.
func (r *Runner) restart(ctx context.Context, ids []string) []ContainerResult {
	if len(ids) == 0 {
		return nil
	}
	r.step("starting %s", redact.Line(strings.Join(ids, ", ")))
	lctx, cancel := context.WithTimeout(ctx, readTimeout)
	items, listErr := r.Client.List(lctx, true)
	cancel()
	exists := make(map[string]bool, len(items))
	for _, it := range items {
		exists[it.ID] = true
	}
	out := make([]ContainerResult, 0, len(ids))
	for _, id := range ids {
		if listErr == nil && !exists[id] {
			out = append(out, ContainerResult{ID: id, Outcome: Gone})
			continue
		}
		sctx, cancel := context.WithTimeout(ctx, startTimeout)
		err := r.Client.Start(sctx, id)
		cancel()
		if err != nil {
			out = append(out, ContainerResult{ID: id, Outcome: StartFailed, Err: err})
			continue
		}
		out = append(out, ContainerResult{ID: id, Outcome: Restarted})
	}
	return out
}

// Write prints the status report. Container names and CLI error text are
// untrusted and go through redact.Line like everything the TUI renders.
func (r Report) Write(w io.Writer) {
	p := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format+"\n", args...) }
	if r.UpToDate {
		p("Apple Container %s is the latest release -- nothing was stopped.", r.Before)
		return
	}
	p("")
	p("Update report")
	switch r.After {
	case "":
		p("  version     %s → unknown (container --version failed)", r.Before)
	case r.Before:
		p("  version     %s (unchanged)", r.Before)
	default:
		p("  version     %s → %s", r.Before, r.After)
	}
	if r.ScriptErr != nil {
		p("  updater     failed: %s", redact.Line(r.ScriptErr.Error()))
	}
	if r.ServiceErr != nil {
		p("  service     did not start: %s", redact.Line(r.ServiceErr.Error()))
		p("              run `container system start` by hand, then start your containers")
		return
	}
	p("  service     %s", redact.Line(r.Service))
	if len(r.Containers) == 0 {
		p("  containers  none were running")
		return
	}
	var restarted, failed, gone int
	for _, c := range r.Containers {
		switch c.Outcome {
		case Restarted:
			restarted++
		case StartFailed:
			failed++
		case Gone:
			gone++
		}
	}
	p("  containers  %d restarted, %d failed, %d gone", restarted, failed, gone)
	for _, c := range r.Containers {
		id := redact.Line(c.ID)
		switch c.Outcome {
		case Restarted:
			p("    ✓ %s  restarted", id)
		case StartFailed:
			p("    ✗ %s  start failed: %s", id, redact.Line(c.Err.Error()))
		case Gone:
			p("    – %s  no longer exists (it ran with --rm)", id)
		}
	}
}
