package metrics

import "github.com/netikras/procfit/internal/model"

// builtinPower are host-scoped power/energy context metrics. C-state residency is
// a CPU-wide property, not per-process (RFC §13/§24): the cstate collector reads
// the host figure once and broadcasts it onto every process, and AggMax over the
// identical values makes any group/rollup show the single host value (never a
// sum). Absent cpuidle → unavailable, never a fabricated zero.
var builtinPower = []Descriptor{
	{ID: "cstate-deep-residency", Unit: UnitPercent, Scope: ScopeHost, Kind: KindGauge,
		Cost: Cost1FD, Collector: "cstate", Aggregation: AggMax,
		Description: "host % of CPU-time in deep idle (C-states deeper than C1), from cpuidle sysfs"},
	// Actual power draw (watts), not a proxy. Host-scoped: read from the first
	// available backend (RAPL powercap / hwmon / battery) and broadcast to every
	// process; a domain the host does not expose renders unavailable, never zero.
	powerDomain("power-pkg", "host CPU package power draw (watts): RAPL/hwmon, else battery"),
	powerDomain("power-core", "host CPU core-domain power draw (watts): RAPL core domain"),
	powerDomain("power-uncore", "host CPU uncore-domain power draw (watts): RAPL uncore domain"),
	powerDomain("power-dram", "host DRAM power draw (watts): RAPL dram domain"),
	powerDomain("power-system", "whole-system power draw (watts): laptop battery (any vendor/arch)"),
}

func powerDomain(id, desc string) Descriptor {
	return Descriptor{
		ID: model.MetricID(id), Unit: UnitWatts, Scope: ScopeHost, Kind: KindGauge,
		Cost: Cost1FD, Collector: "power", Aggregation: AggMax, Description: desc,
	}
}
