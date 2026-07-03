// Package metrics turns the cumulative counters reported by
// `container stats` into per-interval rates.
package metrics

import (
	"time"

	"github.com/eqms/actop/cli"
)

// Sample holds display-ready metrics for one container at one poll tick.
type Sample struct {
	HasRates    bool // false on the first tick for a container
	CPUPercent  float64
	MemUsage    int64
	MemLimit    int64
	NetRxRate   float64 // bytes/s
	NetTxRate   float64
	IOReadRate  float64
	IOWriteRate float64
	PIDs        int
}

type raw struct {
	ts      time.Time
	cpuUsec int64
	netRx   int64
	netTx   int64
	ioRead  int64
	ioWrite int64
}

// Engine caches the previous raw counters per container ID.
// It is used from the Bubble Tea update loop only, so no locking is needed.
type Engine struct {
	prev     map[string]raw
	lastSeen map[string]time.Time
}

// pruneAfter is how long a disappeared container's baseline is kept before
// it is dropped, so the cache does not grow with container churn.
const pruneAfter = 60 * time.Second

func NewEngine() *Engine {
	return &Engine{
		prev:     make(map[string]raw),
		lastSeen: make(map[string]time.Time),
	}
}

// Compute derives a Sample from the current counters. now is the receive
// time of the stats response; nCPU the container's CPU allowance (falls back
// to 1 if <= 0 to avoid division by zero — callers pass host cores instead).
func (e *Engine) Compute(cur cli.StatsEntry, now time.Time, nCPU int) Sample {
	if nCPU <= 0 {
		nCPU = 1
	}
	s := Sample{
		MemUsage: cur.MemoryUsageBytes,
		MemLimit: cur.MemoryLimitBytes,
		PIDs:     cur.NumProcesses,
	}
	p, ok := e.prev[cur.ID]
	e.prev[cur.ID] = raw{
		ts:      now,
		cpuUsec: cur.CPUUsageUsec,
		netRx:   cur.NetworkRxBytes,
		netTx:   cur.NetworkTxBytes,
		ioRead:  cur.BlockReadBytes,
		ioWrite: cur.BlockWriteBytes,
	}
	e.lastSeen[cur.ID] = now
	if !ok {
		return s // first tick: baseline only, no rates
	}
	dt := now.Sub(p.ts).Seconds()
	if dt <= 0 {
		return s
	}
	s.HasRates = true
	s.CPUPercent = float64(clampDelta(cur.CPUUsageUsec, p.cpuUsec)) / 1e6 / dt / float64(nCPU) * 100
	s.NetRxRate = float64(clampDelta(cur.NetworkRxBytes, p.netRx)) / dt
	s.NetTxRate = float64(clampDelta(cur.NetworkTxBytes, p.netTx)) / dt
	s.IOReadRate = float64(clampDelta(cur.BlockReadBytes, p.ioRead)) / dt
	s.IOWriteRate = float64(clampDelta(cur.BlockWriteBytes, p.ioWrite)) / dt
	return s
}

// Prune drops baselines of containers not seen for pruneAfter (e.g. deleted
// or stopped ones), keeping the cache bounded.
func (e *Engine) Prune(now time.Time) {
	for id, seen := range e.lastSeen {
		if now.Sub(seen) > pruneAfter {
			delete(e.prev, id)
			delete(e.lastSeen, id)
		}
	}
}

// clampDelta returns cur-prev, clamped to 0 when the counter reset
// (container restart) would otherwise produce a negative rate.
func clampDelta(cur, prev int64) int64 {
	if d := cur - prev; d > 0 {
		return d
	}
	return 0
}
