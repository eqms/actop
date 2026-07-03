package model

import (
	"time"

	"github.com/eqms/actop/cli"
	"github.com/eqms/actop/metrics"
)

// Store keeps the current container set. It is only accessed from the
// Bubble Tea update loop, so no locking is needed.
type Store struct {
	byID map[string]*Container
}

func NewStore() *Store {
	return &Store{byID: make(map[string]*Container)}
}

// ApplyList replaces the container set with the given list result,
// preserving the last known metrics sample of surviving containers.
func (s *Store) ApplyList(items []cli.ListItem) {
	next := make(map[string]*Container, len(items))
	for _, it := range items {
		c := fromListItem(it)
		if old, ok := s.byID[c.ID]; ok {
			c.Sample = old.Sample
		}
		next[c.ID] = c
	}
	s.byID = next
}

// ApplyStats computes rate samples for all reported containers.
// hostCPUs is the fallback when a container has no CPU limit configured.
func (s *Store) ApplyStats(entries []cli.StatsEntry, now time.Time, engine *metrics.Engine, hostCPUs int) {
	for _, e := range entries {
		c, ok := s.byID[e.ID]
		if !ok {
			continue // stats for a container the last list poll has not seen yet
		}
		nCPU := c.CPUs
		if nCPU <= 0 {
			nCPU = hostCPUs
		}
		c.Sample = engine.Compute(e, now, nCPU)
	}
	engine.Prune(now)
}

// Get returns a container by ID.
func (s *Store) Get(id string) (*Container, bool) {
	c, ok := s.byID[id]
	return c, ok
}

// Snapshot returns the filtered, sorted container list for rendering.
func (s *Store) Snapshot(filter string, field SortField, reversed bool) []*Container {
	out := make([]*Container, 0, len(s.byID))
	for _, c := range s.byID {
		if matchesFilter(c, filter) {
			out = append(out, c)
		}
	}
	sortContainers(out, field, reversed)
	return out
}

// Len returns the number of known containers (unfiltered).
func (s *Store) Len() int { return len(s.byID) }
