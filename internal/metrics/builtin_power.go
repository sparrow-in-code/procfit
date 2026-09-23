package metrics

// builtinPower are host-scoped power/energy context metrics. C-state residency is
// a CPU-wide property, not per-process (RFC §13/§24): the cstate collector reads
// the host figure once and broadcasts it onto every process, and AggMax over the
// identical values makes any group/rollup show the single host value (never a
// sum). Absent cpuidle → unavailable, never a fabricated zero.
var builtinPower = []Descriptor{
	{ID: "cstate-deep-residency", Unit: UnitPercent, Scope: ScopeHost, Kind: KindGauge,
		Cost: Cost1FD, Collector: "cstate", Aggregation: AggMax,
		Description: "host % of CPU-time in deep idle (C-states deeper than C1), from cpuidle sysfs"},
}
