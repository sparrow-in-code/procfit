//go:build !linux

package procfs

import (
	"context"

	"github.com/netikras/procfit/internal/model"
)

// PerfCollector is unsupported off Linux; metrics render unavailable (never
// zero), matching the RFC's optional/degrade-gracefully contract.
type PerfCollector struct{}

// NewPerfCollector builds a stub perf collector.
func NewPerfCollector() *PerfCollector { return &PerfCollector{} }

// ID identifies the collector.
func (c *PerfCollector) ID() string { return "perf" }

// Metrics lists the produced metric ids.
func (c *PerfCollector) Metrics() []model.MetricID {
	return []model.MetricID{"cycles", "instructions", "ipc", "cache-misses", "cache-references"}
}

// Collect marks all perf metrics unsupported on this platform.
func (c *PerfCollector) Collect(_ context.Context, procs []model.Process) {
	for i := range procs {
		for _, id := range c.Metrics() {
			procs[i].SetMetric(id, model.Unavailable[float64](model.Unsupported, "perf"))
		}
	}
}
