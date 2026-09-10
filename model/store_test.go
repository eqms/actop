package model

import (
	"testing"
	"time"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/metrics"
)

func listItem(id, state string, cpus int) cli.ListItem {
	return cli.ListItem{
		ID: id,
		Configuration: cli.Configuration{
			ID:        id,
			Image:     cli.ImageRef{Reference: "docker.io/library/" + id},
			Resources: cli.Resources{CPUs: cpus},
		},
		Status: &cli.Status{
			State:    state,
			Networks: []cli.NetworkAttachment{{IPv4Address: "192.168.64.9/24"}},
		},
	}
}

func TestApplyListReplacesAndPreservesSamples(t *testing.T) {
	s := NewStore()
	s.ApplyList([]cli.ListItem{listItem("a", "running", 2), listItem("b", "stopped", 0)})
	if s.Len() != 2 {
		t.Fatalf("expected 2 containers, got %d", s.Len())
	}

	e := metrics.NewEngine()
	now := time.Now()
	s.ApplyStats([]cli.StatsEntry{{ID: "a", MemoryUsageBytes: 42}}, now, e, 8)
	c, _ := s.Get("a")
	if c.Sample.MemUsage != 42 {
		t.Errorf("sample not applied: %+v", c.Sample)
	}

	// re-list: "b" is gone, "a" keeps its last sample
	s.ApplyList([]cli.ListItem{listItem("a", "running", 2)})
	if s.Len() != 1 {
		t.Fatalf("expected 1 container after re-list, got %d", s.Len())
	}
	c, _ = s.Get("a")
	if c.Sample.MemUsage != 42 {
		t.Error("sample lost across ApplyList")
	}
	if c.IP != "192.168.64.9" {
		t.Errorf("IP not parsed from CIDR: %q", c.IP)
	}
}

func TestSnapshotFilterAndSort(t *testing.T) {
	s := NewStore()
	s.ApplyList([]cli.ListItem{
		listItem("web", "running", 1),
		listItem("db", "running", 1),
		listItem("cache", "stopped", 1),
	})

	all := s.Snapshot("", SortName, false)
	if len(all) != 3 || all[0].ID != "cache" || all[2].ID != "web" {
		t.Errorf("name sort wrong: %v", ids(all))
	}

	rev := s.Snapshot("", SortName, true)
	if rev[0].ID != "web" {
		t.Errorf("reversed sort wrong: %v", ids(rev))
	}

	filtered := s.Snapshot("db", SortName, false)
	if len(filtered) != 1 || filtered[0].ID != "db" {
		t.Errorf("filter wrong: %v", ids(filtered))
	}
}

func TestWebURL(t *testing.T) {
	withPort := &Container{
		IP:    "192.168.64.9",
		Ports: []cli.PublishedPort{{HostPort: 8080, ContainerPort: 80, Proto: "tcp"}},
	}
	if got := withPort.WebURL(); got != "http://localhost:8080" {
		t.Errorf("WebURL with port = %q", got)
	}
	udpOnly := &Container{
		IP:    "192.168.64.9",
		Ports: []cli.PublishedPort{{HostPort: 53, Proto: "udp"}},
	}
	if got := udpOnly.WebURL(); got != "http://192.168.64.9" {
		t.Errorf("WebURL udp-only must fall back to IP, got %q", got)
	}
	if got := (&Container{}).WebURL(); got != "" {
		t.Errorf("WebURL without port/IP must be empty, got %q", got)
	}
}

func ids(cs []*Container) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}
