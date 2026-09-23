package ports

import (
	"context"

	"github.com/netikras/procfit/internal/model"
)

// MetricCollector is a cost>0 collector that enriches already-sampled entities
// with additional metrics (fd/socket, eBPF, perf). It is invoked only when a
// query needs one of its metrics, so disabled collectors do no work (RFC §22).
// Enriching in place keeps the base light sampler independent (Open/Closed:
// adding a collector adds an implementation + registration, no core changes).
type MetricCollector interface {
	// ID names the collector (matches Descriptor.Collector).
	ID() string
	// Metrics lists the metric ids this collector produces.
	Metrics() []model.MetricID
	// Collect enriches the given processes' metric maps in place.
	Collect(ctx context.Context, procs []model.Process)
}

// HostCollector produces host-scoped metrics (ScopeHost) that describe the whole
// machine, not a process — power draw, C-state residency, pressure. They are read
// once per sample into a single map, never broadcast onto every process, and
// rendered in their own section (RFC §13, §24). Adding one is additive.
type HostCollector interface {
	// ID names the collector.
	ID() string
	// Metrics lists the host metric ids this collector produces.
	Metrics() []model.MetricID
	// CollectHost reads the current host values (a metric absent from the map is
	// simply not produced; unavailability is carried in the value).
	CollectHost(ctx context.Context) map[model.MetricID]model.MetricValue
}
