package metrics

import "github.com/netikras/procfit/internal/model"

// builtinPressure are the pressure-stall (PSI) host readouts and per-process
// R/D thread counts — the direct "why is load high" signals (PM-0514). PSI is
// host-scoped (broadcast, AggMax); the thread counts are each process's own
// runnable/uninterruptible contribution to load.
var builtinPressure = []Descriptor{
	psiMetric("psi-cpu", "host CPU pressure: % of time some task stalled on CPU (avg10, /proc/pressure/cpu)"),
	psiMetric("psi-io", "host I/O pressure: % of time some task stalled on I/O (avg10, /proc/pressure/io)"),
	psiMetric("psi-mem", "host memory pressure: % of time some task stalled on memory (avg10, /proc/pressure/memory)"),
	{ID: "threads-running", Unit: UnitInteger, Scope: ScopeProcess, Kind: KindGauge,
		Cost: Cost1FD, Collector: "threadstate", Aggregation: AggSum,
		Description: "count of this process's threads in R (runnable) state — its direct load contribution"},
	{ID: "threads-uninterruptible", Unit: UnitInteger, Scope: ScopeProcess, Kind: KindGauge,
		Cost: Cost1FD, Collector: "threadstate", Aggregation: AggSum,
		Description: "count of this process's threads in D (uninterruptible) state — its I/O-wait load contribution"},
}

func psiMetric(id, desc string) Descriptor {
	return Descriptor{
		ID: model.MetricID(id), Unit: UnitPercent, Scope: ScopeHost, Kind: KindGauge,
		Cost: Cost1FD, Collector: "psi", Aggregation: AggMax, Description: desc,
	}
}

// builtinLoad decompose load average — which is the count of Runnable + Uninter-
// ruptible tasks. blkio-delay is the block-I/O (D-state) contribution; runq-delay
// is the CPU-contention (R-state) contribution. Both are reported as the % of
// wall time spent in that state. blkio-delay is cost-0 (/proc/PID/stat field 42,
// needs kernel delay accounting); runq-delay is cost-1 (/proc/PID/schedstat, the
// "schedstat" collector, needs CONFIG_SCHEDSTATS).
var builtinLoad = []Descriptor{
	{ID: "blkio-delay", Unit: UnitPercent, Scope: ScopeProcess, Kind: KindRate,
		Cost: Cost0Light, Collector: "proc", Aggregation: AggSum, Rate: true,
		Description: "% of wall time blocked on block I/O (needs kernel delay accounting)"},
	{ID: "runq-delay", Unit: UnitPercent, Scope: ScopeProcess, Kind: KindRate,
		Cost: Cost1FD, Collector: "schedstat", Aggregation: AggSum, Rate: true,
		Description: "% of wall time runnable but waiting for a CPU (needs CONFIG_SCHEDSTATS)"},
}
