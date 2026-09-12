//go:build !linux

package procfs

import (
	"context"

	"github.com/netikras/procfit/internal/model"
)

// WakeupsCollector is unsupported off Linux; wakeups renders unavailable.
type WakeupsCollector struct{}

// NewWakeupsCollector builds a stub collector.
func NewWakeupsCollector() *WakeupsCollector { return &WakeupsCollector{} }

// ID matches Descriptor.Collector.
func (c *WakeupsCollector) ID() string { return "ebpf" }

// Metrics lists the produced ids.
func (c *WakeupsCollector) Metrics() []model.MetricID { return []model.MetricID{"wakeups"} }

// Collect marks wakeups unsupported.
func (c *WakeupsCollector) Collect(_ context.Context, procs []model.Process) {
	for i := range procs {
		procs[i].SetMetric("wakeups", model.Unavailable[float64](model.Unsupported, "ebpf"))
	}
}

// Close is a no-op.
func (c *WakeupsCollector) Close() {}
