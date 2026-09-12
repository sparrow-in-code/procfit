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
