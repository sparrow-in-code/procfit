package metrics

import "github.com/netikras/procfit/internal/model"

// builtinMem are memory-depth metrics. The RSS breakdown, peak RSS and swap are
// cost-0 (already-read /proc/PID/status); PSS/USS need /proc/PID/smaps_rollup and
// the OOM score needs /proc/PID/oom_score* — those are cost-1 via the "procmem"
// collector. All stay unavailable (never zero) when their source can't be read.
var builtinMem = []Descriptor{
	memGauge("swap", "proc", Cost0Light, "swapped-out memory (VmSwap)"),
	memGauge("mem-peak", "proc", Cost0Light, "peak resident memory (VmHWM)"),
	memGauge("rss-anon", "proc", Cost0Light, "resident anonymous memory (heap/stack)"),
	memGauge("rss-file", "proc", Cost0Light, "resident file-backed memory"),
	memGauge("rss-shmem", "proc", Cost0Light, "resident shared memory"),
	memGauge("pss", "procmem", Cost1FD, "proportional set size (shared memory attributed fairly)"),
	memGauge("uss", "procmem", Cost1FD, "unique set size (private resident memory)"),
	memGauge("swap-pss", "procmem", Cost1FD, "proportional swapped-out memory"),
	{ID: "oom-score", Unit: UnitInteger, Scope: ScopeProcess, Kind: KindGauge,
		Cost: Cost1FD, Collector: "procmem", Aggregation: AggMax,
		Description: "OOM kill-likelihood score (0–1000; higher is killed first)"},
	{ID: "oom-score-adj", Unit: UnitInteger, Scope: ScopeProcess, Kind: KindGauge,
		Cost: Cost1FD, Collector: "procmem", Aggregation: AggMax,
		Description: "OOM score adjustment (-1000..1000; user/policy bias)"},
}

func memGauge(id, collector string, cost Cost, desc string) Descriptor {
	return Descriptor{
		ID: model.MetricID(id), Unit: UnitBytes, Scope: ScopeProcess, Kind: KindGauge,
		Cost: cost, Collector: collector, Aggregation: AggSumOncePerProc, Description: desc,
	}
}
