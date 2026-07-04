package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestParseListFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/ls.json")
	if err != nil {
		t.Fatal(err)
	}
	var items []ListItem
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal ls fixture: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one container in fixture")
	}
	c := items[0]
	if c.ID == "" {
		t.Error("empty container id")
	}
	if c.Configuration.Image.Reference == "" {
		t.Error("empty image reference")
	}
	if c.Status == nil || c.Status.State == "" {
		t.Error("missing status/state")
	}
	if ip := c.Status.IP(); !strings.HasPrefix(ip, "192.168.") {
		t.Errorf("unexpected IP %q", ip)
	}
	if c.Configuration.Resources.CPUs <= 0 {
		t.Errorf("expected cpus > 0, got %d", c.Configuration.Resources.CPUs)
	}
	if len(c.Configuration.InitProcess.Environment) == 0 {
		t.Error("expected environment entries")
	}

	// the fixture contains one container started with -p 15433:5432
	var withPorts *ListItem
	for i := range items {
		if len(items[i].Configuration.PublishedPorts) > 0 {
			withPorts = &items[i]
		}
	}
	if withPorts == nil {
		t.Fatal("expected a fixture container with published ports")
	}
	p := withPorts.Configuration.PublishedPorts[0]
	if p.HostPort != 15433 || p.ContainerPort != 5432 || p.Proto != "tcp" {
		t.Errorf("unexpected published port: %+v", p)
	}
}

func TestParseStatsFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/stats.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []StatsEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("unmarshal stats fixture: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected stats entries")
	}
	e := entries[0]
	if e.ID == "" || e.CPUUsageUsec <= 0 || e.MemoryUsageBytes <= 0 || e.MemoryLimitBytes <= 0 {
		t.Errorf("implausible stats entry: %+v", e)
	}
}

func TestParseInspectFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/inspect.json")
	if err != nil {
		t.Fatal(err)
	}
	var items []ListItem
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal inspect fixture: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected single-element array, got %d", len(items))
	}
}

func TestParseSystemStatusFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/system_status.json")
	if err != nil {
		t.Fatal(err)
	}
	var st SystemStatus
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("unmarshal system status fixture: %v", err)
	}
	if st.Status != "running" {
		t.Errorf("expected status running, got %q", st.Status)
	}
}

func TestClassifyStderr(t *testing.T) {
	tests := []struct {
		stderr string
		want   error
	}{
		{"Error: XPC connection error: Connection invalid", ErrAPIServerDown},
		{"the container system is not running - run `container system start`", ErrAPIServerDown},
		{`Error: container "foo" not found`, ErrNotFound},
		{"some unrelated failure", nil},
	}
	for _, tt := range tests {
		if got := classifyStderr(tt.stderr); got != tt.want {
			t.Errorf("classifyStderr(%q) = %v, want %v", tt.stderr, got, tt.want)
		}
	}
}
