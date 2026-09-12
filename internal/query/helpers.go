package query

import (
	"fmt"

	"github.com/netikras/procfit/internal/model"
)

// unknownKey is the bucket key for entities whose dimension value is
// unavailable, keeping "unknown" distinct rather than fabricating a value.
const unknownKey = "\x00unknown"

// bucketBy groups processes by a dimension, returning keys in first-seen order
// for determinism (final ordering is decided by sort).
func bucketBy(procs []*model.Process, dim Dimension) ([]string, map[string][]*model.Process) {
	order := make([]string, 0)
	buckets := make(map[string][]*model.Process)
	for _, p := range procs {
		key, _, ok := dim.Key(p)
		if !ok {
			key = unknownKey
		}
		if _, seen := buckets[key]; !seen {
			order = append(order, key)
		}
		buckets[key] = append(buckets[key], p)
	}
	return order, buckets
}

func bucketLabel(dim Dimension, procs []*model.Process, key string) string {
	if key == unknownKey {
		return fmt.Sprintf("%s:?", dim.ID)
	}
	if len(procs) > 0 {
		if _, label, ok := dim.Key(procs[0]); ok {
			return label
		}
	}
	return key
}

func leafLabel(p *model.Process) string {
	if n := p.DisplayName(); n != "" {
		return n
	}
	return fmt.Sprintf("pid:%d", p.PID)
}

// procThreadCount returns the thread count for a process, preferring enumerated
// threads when present.
func procThreadCount(p *model.Process) int {
	if len(p.Threads) > 0 {
		return len(p.Threads)
	}
	return p.NumThreads
}

// projectMetrics copies the requested metric ids, substituting an
// unavailable placeholder for any the entity lacks (never a zero).
func projectMetrics(src map[model.MetricID]model.MetricValue, ids []model.MetricID) map[model.MetricID]model.MetricValue {
	out := make(map[model.MetricID]model.MetricValue, len(ids))
	for _, id := range ids {
		if v, ok := src[id]; ok {
			out[id] = v
		} else {
			out[id] = model.Unavailable[float64](model.Disabled, "")
		}
	}
	return out
}
