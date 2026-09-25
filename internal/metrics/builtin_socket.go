package metrics

import "github.com/netikras/procfit/internal/model"

// builtinSocket are per-process socket classification metrics (RFC §13.2),
// derived without eBPF/root by joining /proc/PID/fd socket inodes with
// /proc/net/{tcp,udp,unix}. Counts carry Sampled quality (the set can change
// mid-scan); sockets in another netns or of other families are simply not
// counted, never fabricated as zero when unavailable.
var builtinSocket = []Descriptor{
	sockGauge("sock-tcp", "TCP sockets held by the process"),
	sockGauge("sock-udp", "UDP sockets held by the process"),
	sockGauge("sock-unix", "UNIX-domain sockets held by the process"),
	sockGauge("sock-listen", "listening TCP sockets (server side)"),
	sockGauge("sock-estab", "established TCP connections"),
	sockGauge("sock-timewait", "TCP sockets in TIME_WAIT"),
	sockGauge("sock-closewait", "TCP sockets in CLOSE_WAIT"),
}

func sockGauge(id, desc string) Descriptor {
	return Descriptor{
		ID: model.MetricID(id), Unit: UnitCount, Scope: ScopeProcess, Kind: KindGauge,
		Cost: Cost1FD, Collector: "socket", Aggregation: AggSum, Description: desc,
	}
}
