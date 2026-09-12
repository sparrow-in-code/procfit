package query

import (
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
)

// aggregateMetrics computes group metric values across the given processes,
// honouring each metric's declared aggregation and deduplicating process-scoped
// values so thread enumeration cannot double count (RFC §12.3, §12.4).
//
// Because grouping here buckets whole processes (each process appears once per
// group), sum and sum-once-per-process coincide at this level; the distinction
// matters once thread leaves inherit process-scoped values, which are marked
// display-only and never summed.
func aggregateMetrics(reg *metrics.Registry, procs []*model.Process, ids []model.MetricID) map[model.MetricID]model.MetricValue {
	out := make(map[model.MetricID]model.MetricValue, len(ids))
	for _, id := range ids {
		desc, ok := reg.Get(string(id))
		agg := metrics.AggSum
		if ok {
			agg = desc.Aggregation
		}
		out[id] = aggregateOne(agg, id, procs)
	}
	return out
}

func aggregateOne(agg metrics.Aggregation, id model.MetricID, procs []*model.Process) model.MetricValue {
	vals := presentValues(procs, id)
	if len(vals) == 0 {
		return model.Unavailable[float64](model.Disabled, "aggregate")
	}
	switch agg {
	case metrics.AggMin:
		return model.NewValue(reduce(vals, minf), model.Derived, "aggregate")
	case metrics.AggMax:
		return model.NewValue(reduce(vals, maxf), model.Derived, "aggregate")
	case metrics.AggMixed:
		return mixedValue(vals)
	default: // sum, sum-once-per-process, and fallbacks
		return model.NewValue(reduce(vals, addf), model.Derived, "aggregate")
	}
}

func presentValues(procs []*model.Process, id model.MetricID) []float64 {
	var out []float64
	for _, p := range procs {
		if val, ok := p.Metric(id).Get(); ok {
			out = append(out, val)
		}
	}
	return out
}

func reduce(vals []float64, f func(a, b float64) float64) float64 {
	acc := vals[0]
	for _, v := range vals[1:] {
		acc = f(acc, v)
	}
	return acc
}

func addf(a, b float64) float64 { return a + b }
func minf(a, b float64) float64 {
	if b < a {
		return b
	}
	return a
}
func maxf(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}

// mixedValue returns the uniform value, or a "mixed" marker when a group holds
// differing values (RFC §12.3, §15.3): rendered as "mixed", never a fake number.
func mixedValue(vals []float64) model.MetricValue {
	first := vals[0]
	for _, v := range vals[1:] {
		if v != first {
			return model.Value[float64]{V: first, Availability: model.Available, Quality: "mixed", Source: "aggregate"}
		}
	}
	return model.NewValue(first, model.Derived, "aggregate")
}
