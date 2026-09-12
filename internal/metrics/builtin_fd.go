package metrics

import "github.com/netikras/procfit/internal/model"

// builtinFD are cost-level-1 file-descriptor/socket classification metrics (RFC
// §13.2), collected by periodic /proc/PID/fd scans. Counts carry sampled quality.
var builtinFD = []Descriptor{
	fdGauge("fd-total", "total open file descriptors"),
	fdGauge("fd-files", "regular-file descriptors"),
	fdGauge("fd-sockets", "socket descriptors"),
	fdGauge("fd-pipes", "pipe descriptors"),
	fdGauge("fd-anon", "anonymous-inode descriptors"),
	fdGauge("fd-eventfd", "eventfd descriptors"),
	fdGauge("fd-epoll", "epoll descriptors"),
	fdGauge("fd-timerfd", "timerfd descriptors"),
	fdGauge("fd-signalfd", "signalfd descriptors"),
	fdGauge("fd-inotify", "inotify descriptors"),
}

func fdGauge(id, desc string) Descriptor {
	return Descriptor{
		ID: model.MetricID(id), Unit: UnitCount, Scope: ScopeProcess, Kind: KindGauge,
		Cost: Cost1FD, Collector: "fd", Aggregation: AggSum, Description: desc,
	}
}
