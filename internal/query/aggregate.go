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
	var acc float64
	var count int
	var haveMinMax bool
	var uniform = true
	var first float64
	for _, p := range procs {
		v := p.Metric(id)
		val, ok := v.Get()
		if !ok {
			continue
		}
		if count == 0 {
			first = val
		} else if val != first {
			uniform = false
		}
		switch agg {
		case metrics.AggMin:
			if !haveMinMax || val < acc {
				acc = val
			}
		case metrics.AggMax:
			if !haveMinMax || val > acc {
				acc = val
			}
		default: // sum, sum-once-per-process, and fallbacks
			acc += val
		}
		haveMinMax = true
		count++
	}
	if count == 0 {
		return model.Unavailable[float64](model.Disabled, "aggregate")
	}
	if agg == metrics.AggMixed {
		if uniform {
			return model.NewValue(first, model.Derived, "aggregate")
		}
		// Mixed group: no single numeric value is meaningful; renderers show
		// "mixed" (RFC §12.3, §15.3). Surface as unavailable-with-reason so it
		// is never mistaken for a real number.
		return model.Value[float64]{V: first, Availability: model.Available, Quality: "mixed", Source: "aggregate"}
	}
	return model.NewValue(acc, model.Derived, "aggregate")
}
