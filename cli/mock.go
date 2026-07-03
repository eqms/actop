package cli

import (
	"context"
	"os/exec"
)

// MockClient implements Client for tests. Set the Fn fields you need;
// unset functions return zero values. Every call is recorded in Calls.
type MockClient struct {
	ListFn         func(ctx context.Context, all bool) ([]ListItem, error)
	StatsFn        func(ctx context.Context) ([]StatsEntry, error)
	InspectFn      func(ctx context.Context, id string) (*ListItem, error)
	SystemStatusFn func(ctx context.Context) (*SystemStatus, error)
	ActionFn       func(ctx context.Context, action, id string) error

	Calls []MockCall
}

type MockCall struct {
	Method string
	ID     string
}

func (m *MockClient) record(method, id string) {
	m.Calls = append(m.Calls, MockCall{Method: method, ID: id})
}

func (m *MockClient) List(ctx context.Context, all bool) ([]ListItem, error) {
	m.record("List", "")
	if m.ListFn != nil {
		return m.ListFn(ctx, all)
	}
	return nil, nil
}

func (m *MockClient) Stats(ctx context.Context) ([]StatsEntry, error) {
	m.record("Stats", "")
	if m.StatsFn != nil {
		return m.StatsFn(ctx)
	}
	return nil, nil
}

func (m *MockClient) Inspect(ctx context.Context, id string) (*ListItem, error) {
	m.record("Inspect", id)
	if m.InspectFn != nil {
		return m.InspectFn(ctx, id)
	}
	return nil, ErrNotFound
}

func (m *MockClient) SystemStatus(ctx context.Context) (*SystemStatus, error) {
	m.record("SystemStatus", "")
	if m.SystemStatusFn != nil {
		return m.SystemStatusFn(ctx)
	}
	return &SystemStatus{Status: "running"}, nil
}

func (m *MockClient) action(ctx context.Context, action, id string) error {
	m.record(action, id)
	if m.ActionFn != nil {
		return m.ActionFn(ctx, action, id)
	}
	return nil
}

func (m *MockClient) Start(ctx context.Context, id string) error  { return m.action(ctx, "Start", id) }
func (m *MockClient) Stop(ctx context.Context, id string) error   { return m.action(ctx, "Stop", id) }
func (m *MockClient) Kill(ctx context.Context, id string) error   { return m.action(ctx, "Kill", id) }
func (m *MockClient) Delete(ctx context.Context, id string) error { return m.action(ctx, "Delete", id) }

func (m *MockClient) LogsCmd(id string, follow bool) *exec.Cmd {
	return exec.Command("cat", "/dev/null")
}

func (m *MockClient) ExecCmd(id string, shell ...string) *exec.Cmd {
	return exec.Command("true")
}
