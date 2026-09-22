package metrics

import "github.com/netikras/procfit/internal/model"

// builtinPerf are cost-level-3 hardware performance-counter metrics (RFC §13.4),
// backed by perf_event_open. Registered for profile/column resolution; they stay
// unavailable until a perf backend is active and permitted.
var builtinPerf = []Descriptor{
	perfGauge("cycles", UnitCount, "CPU cycles per second"),
	perfGauge("instructions", UnitCount, "instructions retired per second"),
	perfGauge("ipc", UnitRatio, "instructions per cycle (higher is better; <1 is stall-heavy)"),
	perfGauge("cache-references", UnitCount, "last-level cache references per second"),
	perfGauge("cache-misses", UnitCount, "last-level cache misses per second (pair with cache-references for miss rate)"),
}

func perfGauge(id string, unit Unit, desc string) Descriptor {
	return Descriptor{
		ID: model.MetricID(id), Unit: unit, Scope: ScopeProcess, Kind: KindGauge,
		Cost: Cost3Perf, Collector: "perf", Aggregation: AggSum, Description: desc,
	}
}
