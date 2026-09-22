// Package metrics is the registry of metric descriptors and profiles. Metric
// behaviour (unit, scope, cost, aggregation, delta semantics) is data carried on
// the descriptor, never logic embedded in collectors or renderers (RFC §12.3,
// §13, §25). Adding a metric means adding a descriptor + a collector — no query
// or render code changes (Open/Closed, DEVELOPMENT.md §2.2).
package metrics

import "github.com/netikras/procfit/internal/model"

// Unit describes a metric's unit for formatting and type-checking.
type Unit string

const (
	UnitPercentOneCPU Unit = "percent_one_cpu" // may exceed 100 for multithreaded
	UnitPercentHost   Unit = "percent_host"
	UnitPercent       Unit = "percent" // 0–100 utilization (may exceed across GPU engines)
	UnitBytes         Unit = "bytes"
	UnitBytesPerSec   Unit = "bytes_per_sec"
	UnitCount         Unit = "count"
	UnitPerSec        Unit = "per_sec"
	UnitDuration      Unit = "duration"
	UnitInteger       Unit = "integer"
	UnitEnum          Unit = "enum"
)

// Scope tags where a metric's value is accounted, so aggregation can deduplicate
// process-scoped values under thread leaves (RFC §12.4).
type Scope string

const (
	ScopeHost    Scope = "host"
	ScopeProcess Scope = "process"
	ScopeThread  Scope = "thread"
)

// Kind is the value kind of a metric.
type Kind string

const (
	KindGauge   Kind = "gauge"   // absolute value (rss)
	KindCounter Kind = "counter" // monotonic; rate is a delta over time
	KindRate    Kind = "rate"    // derived per-second rate
	KindEnum    Kind = "enum"    // categorical (pstate)
)

// Cost is the collection cost level (RFC §13).
type Cost int

const (
	Cost0Light Cost = 0 // cheap /proc reads
	Cost1FD    Cost = 1 // periodic fd/socket scans
	Cost2Event Cost = 2 // eBPF/event tracing
	Cost3Perf  Cost = 3 // perf/hardware counters
)

// Aggregation is how a metric combines across a group's members (RFC §12.3).
type Aggregation string

const (
	AggSum            Aggregation = "sum"
	AggSumOncePerProc Aggregation = "sum_once_per_process" // rss/vsz dedup by process
	AggMin            Aggregation = "min"
	AggMax            Aggregation = "max"
	AggMixed          Aggregation = "mixed" // nice: min/max/mixed display, never summed
	AggNone           Aggregation = "none"
)

// Descriptor is the immutable metadata for a metric.
type Descriptor struct {
	ID          model.MetricID
	Aliases     []string
	Unit        Unit
	Scope       Scope
	Kind        Kind
	Cost        Cost
	Collector   string
	Aggregation Aggregation
	// Rate indicates the metric is reported as a per-second rate derived from a
	// counter, which requires two samples (warm-up) before a value exists.
	Rate        bool
	Description string
}

// IsRate reports whether the metric requires two samples to produce a value.
func (d Descriptor) IsRate() bool { return d.Rate || d.Kind == KindRate }
