package upgrade

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eqms/actop/cli"
)

// fixture wires a Runner to a MockClient. running is what `list` reports
// before the update; exists is what `list --all` reports after the service
// is back, so a --rm container can be made to disappear.
type fixture struct {
	mock      *cli.MockClient
	runner    *Runner
	out       *bytes.Buffer
	scriptRan bool
	asked     string
}

func newFixture(t *testing.T, running, exists []string) *fixture {
	t.Helper()
	script := filepath.Join(t.TempDir(), "update-container.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &fixture{mock: &cli.MockClient{VersionValue: "1.3.1"}, out: &bytes.Buffer{}}
	f.mock.ListFn = func(_ context.Context, all bool) ([]cli.ListItem, error) {
		ids := running
		state := "running"
		if all {
			ids, state = exists, "stopped"
		}
		items := make([]cli.ListItem, 0, len(ids))
		for _, id := range ids {
			items = append(items, cli.ListItem{ID: id, Status: &cli.Status{State: state}})
		}
		return items, nil
	}
	f.mock.SystemStatusFn = func(context.Context) (*cli.SystemStatus, error) {
		return &cli.SystemStatus{Status: "running"}, nil
	}
	f.runner = &Runner{
		Client: f.mock,
		Latest: func(context.Context) (string, bool) { return "1.4.1", true },
		Script: script,
		RunScript: func(context.Context, string) error {
			f.scriptRan = true
			f.mock.VersionValue = "1.4.1"
			return nil
		},
		Confirm: func(q string) bool { f.asked = q; return true },
		Out:     f.out,
	}
	return f
}

// calls lists the recorded client calls that change state, in order.
func (f *fixture) calls() []string {
	var out []string
	for _, c := range f.mock.CallList() {
		switch c.Method {
		case "SystemStop", "SystemStart":
			out = append(out, c.Method)
		case "Start", "Stop", "Delete", "Kill":
			out = append(out, c.Method+" "+c.ID)
		}
	}
	return out
}

func (f *fixture) failOn(method, id string) {
	f.mock.ActionFn = func(_ context.Context, m, i string) error {
		if m == method && i == id {
			return errors.New(method + " failed")
		}
		return nil
	}
}

func TestFullRunRestartsWhatWasRunning(t *testing.T) {
	f := newFixture(t, []string{"web", "db"}, []string{"web", "db", "old"})
	rep, err := f.runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{"SystemStop", "SystemStart", "Start web", "Start db"}
	if got := f.calls(); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
	if !f.scriptRan {
		t.Error("Apple's updater never ran")
	}
	if rep.Before != "1.3.1" || rep.After != "1.4.1" || rep.Service != "running" {
		t.Errorf("report = %+v", rep)
	}
	for _, c := range rep.Containers {
		if c.Outcome != Restarted {
			t.Errorf("%s: outcome %v, want Restarted", c.ID, c.Outcome)
		}
	}
	if len(rep.Containers) != 2 || !rep.OK() {
		t.Errorf("containers = %+v, OK = %v", rep.Containers, rep.OK())
	}
	if !strings.Contains(f.asked, "web") || !strings.Contains(f.asked, "db") {
		t.Errorf("question %q must name the containers that will stop", f.asked)
	}
}

func TestUpToDateStopsNothing(t *testing.T) {
	f := newFixture(t, []string{"web"}, []string{"web"})
	f.runner.Latest = func(context.Context) (string, bool) { return "1.3.1", true }
	rep, err := f.runner.Run(context.Background())
	if err != nil || !rep.UpToDate {
		t.Fatalf("Run = (%+v, %v), want up to date", rep, err)
	}
	if got := f.calls(); len(got) != 0 || f.scriptRan {
		t.Errorf("up to date must touch nothing, calls = %v, script ran = %v", got, f.scriptRan)
	}
}

// Without the release lookup there is no knowing whether the update is
// needed -- and the updater would fail on the same network anyway.
func TestLookupFailureStopsNothing(t *testing.T) {
	f := newFixture(t, []string{"web"}, []string{"web"})
	f.runner.Latest = func(context.Context) (string, bool) { return "", false }
	if _, err := f.runner.Run(context.Background()); err == nil {
		t.Fatal("a failed lookup must be an error")
	}
	if got := f.calls(); len(got) != 0 || f.scriptRan {
		t.Errorf("calls = %v, script ran = %v, want nothing", got, f.scriptRan)
	}
}

func TestMissingScriptStopsNothing(t *testing.T) {
	f := newFixture(t, []string{"web"}, []string{"web"})
	f.runner.Script = filepath.Join(t.TempDir(), "absent.sh")
	_, err := f.runner.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "absent.sh") {
		t.Fatalf("err = %v, want one naming the missing updater", err)
	}
	if got := f.calls(); len(got) != 0 {
		t.Errorf("calls = %v, want nothing", got)
	}
}

func TestDeclineStopsNothing(t *testing.T) {
	f := newFixture(t, []string{"web"}, []string{"web"})
	f.runner.Confirm = func(string) bool { return false }
	if _, err := f.runner.Run(context.Background()); !errors.Is(err, ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
	if got := f.calls(); len(got) != 0 || f.scriptRan {
		t.Errorf("calls = %v, script ran = %v, want nothing", got, f.scriptRan)
	}
}

// Stopping an idle service costs nothing, so there is nothing to ask about.
func TestNothingRunningSkipsQuestion(t *testing.T) {
	f := newFixture(t, nil, nil)
	f.runner.Confirm = func(string) bool { t.Error("asked although nothing runs"); return false }
	if _, err := f.runner.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !f.scriptRan {
		t.Error("updater did not run")
	}
}

func TestStopFailureAbortsBeforeUpdater(t *testing.T) {
	f := newFixture(t, []string{"web"}, []string{"web"})
	f.failOn("SystemStop", "")
	if _, err := f.runner.Run(context.Background()); err == nil {
		t.Fatal("a failed service stop must be an error")
	}
	if f.scriptRan {
		t.Error("updater ran although the service is still up")
	}
}

// A failed update must not leave the machine without its containers.
func TestUpdaterFailureStillRestarts(t *testing.T) {
	f := newFixture(t, []string{"web"}, []string{"web"})
	f.runner.RunScript = func(context.Context, string) error { return errors.New("installer failed") }
	rep, err := f.runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{"SystemStop", "SystemStart", "Start web"}
	if got := f.calls(); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
	if rep.ScriptErr == nil || rep.OK() {
		t.Errorf("report must carry the updater failure: %+v", rep)
	}
}

func TestServiceStartFailureSkipsContainers(t *testing.T) {
	f := newFixture(t, []string{"web"}, []string{"web"})
	f.failOn("SystemStart", "")
	rep, err := f.runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := f.calls(); !slices.Equal(got, []string{"SystemStop", "SystemStart"}) {
		t.Errorf("calls = %v, want no container starts", got)
	}
	if rep.ServiceErr == nil || rep.OK() {
		t.Errorf("report must carry the service failure: %+v", rep)
	}
}

// Apple deletes --rm containers when the service stops; they cannot come back.
func TestRemovedContainerReportedGone(t *testing.T) {
	f := newFixture(t, []string{"web", "tmp"}, []string{"web"})
	rep, _ := f.runner.Run(context.Background())
	if got := f.calls(); slices.Contains(got, "Start tmp") {
		t.Errorf("tried to start a container that no longer exists: %v", got)
	}
	var gone bool
	for _, c := range rep.Containers {
		if c.ID == "tmp" && c.Outcome == Gone {
			gone = true
		}
	}
	if !gone {
		t.Errorf("tmp not reported gone: %+v", rep.Containers)
	}
	if !rep.OK() {
		t.Error("a --rm container that is gone is expected, not a failure")
	}
}

func TestStartFailureReported(t *testing.T) {
	f := newFixture(t, []string{"web", "db"}, []string{"web", "db"})
	f.failOn("Start", "db")
	rep, _ := f.runner.Run(context.Background())
	if got := f.calls(); !slices.Contains(got, "Start web") {
		t.Errorf("one failure must not stop the others: %v", got)
	}
	for _, c := range rep.Containers {
		if c.ID == "db" && (c.Outcome != StartFailed || c.Err == nil) {
			t.Errorf("db = %+v, want StartFailed with error", c)
		}
	}
	if rep.OK() {
		t.Error("a failed start must fail the report")
	}
}

func TestReportWrite(t *testing.T) {
	rep := Report{
		Before: "1.3.1", After: "1.4.1", Service: "running",
		Containers: []ContainerResult{
			{ID: "web", Outcome: Restarted},
			{ID: "db", Outcome: StartFailed, Err: errors.New("no space")},
			{ID: "tmp\x1b]52;c;cGF5bG9hZA==\x07", Outcome: Gone},
		},
	}
	var b bytes.Buffer
	rep.Write(&b)
	out := b.String()
	for _, want := range []string{"1.3.1 → 1.4.1", "running", "web", "restarted",
		"db", "no space", "tmp", "no longer exists"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
	if strings.ContainsRune(out, '\x1b') || strings.ContainsRune(out, '\x07') {
		t.Errorf("container names must be sanitized:\n%q", out)
	}
}
