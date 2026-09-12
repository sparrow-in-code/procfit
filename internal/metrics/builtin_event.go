package metrics

import "github.com/netikras/procfit/internal/model"

// builtinEvent are cost-level-2 event-tracing metrics (RFC §13.3), normally
// backed by eBPF. They are registered so profiles and columns resolve, but
// remain unavailable until an eBPF backend is active (never fabricated as zero).
var builtinEvent = []Descriptor{
	eventRate("wakeups", "scheduler wakeups per second (awakened-task attribution)"),
	eventRate("timer-wakeups", "timer-driven wakeups per second"),
	eventRate("net-rx-bps", "per-process received bytes/sec (requires eBPF/accounting)"),
	eventRate("net-tx-bps", "per-process transmitted bytes/sec (requires eBPF/accounting)"),
	eventRate("net-rx-pps", "per-process received packets/sec"),
	eventRate("net-tx-pps", "per-process transmitted packets/sec"),
}

func eventRate(id, desc string) Descriptor {
	return Descriptor{
		ID: model.MetricID(id), Unit: UnitPerSec, Scope: ScopeProcess, Kind: KindRate,
		Cost: Cost2Event, Collector: "ebpf", Aggregation: AggSum, Rate: true, Description: desc,
	}
}
