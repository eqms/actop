package ui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/config"
)

// syncBuffer collects renderer output from the program's goroutine while the
// test reads it, so the race detector stays quiet.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestProgramRendersAndQuits drives the real Bubble Tea program end to end: it
// boots, polls, paints the container into a frame and exits on "q". Unit tests
// call view() directly and would not catch a program-level regression -- the
// View contract and alt-screen handling both moved in the v1 -> v2 upgrade.
func TestProgramRendersAndQuits(t *testing.T) {
	mock := &cli.MockClient{
		ListFn: func(context.Context, bool) ([]cli.ListItem, error) {
			return []cli.ListItem{{
				ID:     "smoke-db",
				Status: &cli.Status{State: "running"},
			}}, nil
		},
	}
	app := NewApp(mock, config.Default(), "test")

	// Quit only once the polled container has actually been painted, so the
	// assertion is about rendering rather than about winning a race.
	keys, keysIn := io.Pipe()
	out := &syncBuffer{}

	p := tea.NewProgram(app,
		tea.WithInput(keys),
		tea.WithOutput(out),
		tea.WithWindowSize(120, 40),
	)

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(out.String(), "smoke-db") {
		if time.Now().After(deadline) {
			p.Kill()
			t.Fatalf("container never appeared in a frame; got:\n%s", out.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := keysIn.Write([]byte("q")); err != nil {
		t.Fatalf("writing key: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program returned %v", err)
		}
	case <-time.After(10 * time.Second):
		p.Kill()
		t.Fatal("program did not quit on \"q\"")
	}
	_ = keysIn.Close()

	frame := out.String()
	for _, want := range []string{"actop vtest", "containers", "smoke-db", "running"} {
		if !strings.Contains(frame, want) {
			t.Errorf("rendered frame is missing %q", want)
		}
	}
}

// compile-time assurance that App still satisfies the root model contract
var _ tea.Model = (*App)(nil)
