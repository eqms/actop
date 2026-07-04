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
