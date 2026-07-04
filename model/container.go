// Package model holds the UI-independent domain state: containers merged
// from list and stats polls, plus sorting and filtering.
package model

import (
	"fmt"
	"time"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/metrics"
)

// Container is the merged view of one Apple container. Its ID doubles as
// the user-visible name (Apple Container has no separate name field).
type Container struct {
	ID      string
	Image   string
	State   string
	Created time.Time
	Started time.Time
	IP      string
	CPUs    int
	Env     []string
	Ports   []cli.PublishedPort
	Sample  metrics.Sample
	History *History
}

// WebURL returns the address to open in a browser: the first published
// TCP port on localhost (macOS forwards it), else the container's vmnet IP.
func (c *Container) WebURL() string {
	for _, p := range c.Ports {
		if p.Proto == "" || p.Proto == "tcp" {
			return fmt.Sprintf("http://localhost:%d", p.HostPort)
		}
	}
	if c.IP != "" {
		return "http://" + c.IP
	}
	return ""
}

// History keeps recent metric samples for sparkline rendering.
// CPU holds percent values; Mem holds usage as percent of the limit.
type History struct {
	CPU *metrics.RingBuffer
	Mem *metrics.RingBuffer
}

func newHistory() *History {
	return &History{
		CPU: metrics.NewRingBuffer(metrics.HistorySize),
		Mem: metrics.NewRingBuffer(metrics.HistorySize),
	}
}

func fromListItem(it cli.ListItem) *Container {
	c := &Container{
		ID:      it.ID,
		Image:   it.Configuration.Image.Reference,
		Created: it.Configuration.CreationDate,
		CPUs:    it.Configuration.Resources.CPUs,
		Env:     it.Configuration.InitProcess.Environment,
		Ports:   it.Configuration.PublishedPorts,
	}
	if it.Status != nil {
		c.State = it.Status.State
		c.Started = it.Status.StartedDate
		c.IP = it.Status.IP()
	}
	return c
}

// Running reports whether the container is currently running.
func (c *Container) Running() bool { return c.State == "running" }

// Uptime returns the time since start for running containers, else 0.
func (c *Container) Uptime(now time.Time) time.Duration {
	if !c.Running() || c.Started.IsZero() {
		return 0
	}
	return now.Sub(c.Started)
}
