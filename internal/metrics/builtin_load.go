package metrics

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
