//go:build linux

package procfs

import (
	"context"
	"os"
	"testing"

	"github.com/netikras/procfit/internal/model"
)

// These exercise the perf and eBPF collectors' code paths regardless of
// privilege: as root they produce real values; unprivileged they degrade to
// unavailable. Either way every metric must be SET (never left absent), so the
// test is deterministic without asserting a specific privilege level.

func TestPerfCollector_PopulatesMetrics(t *testing.T) {
	c := NewPerfCollector()
	procs := []model.Process{{PID: os.Getpid()}}
	c.Collect(context.Background(), procs)
	for _, id := range c.Metrics() {
		if _, ok := procs[0].Metrics[id]; !ok {
			t.Fatalf("perf metric %q not set", id)
		}
	}
	if c.ID() != "perf" {
		t.Fatalf("perf ID = %q", c.ID())
	}
}

func TestWakeupsCollector_PopulatesMetric(t *testing.T) {
	c := NewWakeupsCollector()
	defer c.Close()
	if c.ID() != "ebpf" {
		t.Fatalf("wakeups ID = %q", c.ID())
	}
	procs := []model.Process{{PID: os.Getpid()}}
	c.Collect(context.Background(), procs)
	if _, ok := procs[0].Metrics["wakeups"]; !ok {
		t.Fatal("wakeups metric not set on first collect")
	}
	// Second call covers the already-loaded (root) or already-failed path.
	c.Collect(context.Background(), procs)
	if _, ok := procs[0].Metrics["wakeups"]; !ok {
		t.Fatal("wakeups metric not set on second collect")
	}
}
