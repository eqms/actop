package metrics

import (
	"math"
	"testing"
	"time"

	"github.com/eqms/actop/cli"
)

func entry(id string, cpuUsec, rx, tx, rd, wr int64, pids int) cli.StatsEntry {
	return cli.StatsEntry{
		ID: id, CPUUsageUsec: cpuUsec,
		MemoryUsageBytes: 100, MemoryLimitBytes: 200,
		NetworkRxBytes: rx, NetworkTxBytes: tx,
		BlockReadBytes: rd, BlockWriteBytes: wr,
		NumProcesses: pids,
	}
}

func TestFirstTickHasNoRates(t *testing.T) {
	e := NewEngine()
	s := e.Compute(entry("a", 1000, 10, 10, 10, 10, 3), time.Now(), 4)
	if s.HasRates {
		t.Error("first tick must not report rates")
	}
	if s.MemUsage != 100 || s.PIDs != 3 {
		t.Errorf("absolute values missing: %+v", s)
	}
}

func TestRates(t *testing.T) {
	e := NewEngine()
	t0 := time.Now()
	e.Compute(entry("a", 0, 0, 0, 0, 0, 1), t0, 4)
	// 2s later: 4_000_000 usec CPU on 4 CPUs = 50%; 2048 bytes rx = 1024 B/s
	s := e.Compute(entry("a", 4_000_000, 2048, 4096, 8192, 16384, 1), t0.Add(2*time.Second), 4)
	if !s.HasRates {
		t.Fatal("expected rates on second tick")
	}
	if math.Abs(s.CPUPercent-50.0) > 0.01 {
		t.Errorf("CPUPercent = %f, want 50", s.CPUPercent)
	}
	if math.Abs(s.NetRxRate-1024) > 0.01 || math.Abs(s.NetTxRate-2048) > 0.01 {
		t.Errorf("net rates = %f/%f, want 1024/2048", s.NetRxRate, s.NetTxRate)
	}
	if math.Abs(s.IOReadRate-4096) > 0.01 || math.Abs(s.IOWriteRate-8192) > 0.01 {
		t.Errorf("io rates = %f/%f, want 4096/8192", s.IOReadRate, s.IOWriteRate)
	}
}

func TestCounterResetClampsToZero(t *testing.T) {
	e := NewEngine()
	t0 := time.Now()
	e.Compute(entry("a", 9_000_000, 900, 900, 900, 900, 1), t0, 2)
	s := e.Compute(entry("a", 1000, 10, 10, 10, 10, 1), t0.Add(2*time.Second), 2)
	if s.CPUPercent != 0 || s.NetRxRate != 0 || s.IOWriteRate != 0 {
		t.Errorf("negative deltas must clamp to 0: %+v", s)
	}
}

func TestZeroDtGuard(t *testing.T) {
	e := NewEngine()
	t0 := time.Now()
	e.Compute(entry("a", 0, 0, 0, 0, 0, 1), t0, 1)
	s := e.Compute(entry("a", 1000, 10, 10, 10, 10, 1), t0, 1)
	if s.HasRates {
		t.Error("dt=0 must not report rates")
	}
}

func TestNCPUFallback(t *testing.T) {
	e := NewEngine()
	t0 := time.Now()
	e.Compute(entry("a", 0, 0, 0, 0, 0, 1), t0, 0)
	s := e.Compute(entry("a", 1_000_000, 0, 0, 0, 0, 1), t0.Add(time.Second), 0)
	if math.Abs(s.CPUPercent-100.0) > 0.01 {
		t.Errorf("nCPU<=0 should fall back to 1: got %f", s.CPUPercent)
	}
}

func TestPrune(t *testing.T) {
	e := NewEngine()
	t0 := time.Now()
	e.Compute(entry("a", 0, 0, 0, 0, 0, 1), t0, 1)
	e.Prune(t0.Add(2 * pruneAfter))
	if len(e.prev) != 0 || len(e.lastSeen) != 0 {
		t.Error("stale entries not pruned")
	}
}
