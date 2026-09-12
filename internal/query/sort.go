package query

import (
	"sort"

	"github.com/netikras/procfit/internal/model"
)

// sortTree sorts siblings at every level independently (RFC §8.4). Sorting is
// stable; unavailable values sort last regardless of direction; deterministic
// tie-breakers are row kind, then display label, then stable key.
func (e *Engine) sortTree(rows []*Row, keys []SortKey) {
	sortRows(rows, keys)
	for _, r := range rows {
		e.sortTree(r.Sub, keys)
	}
}

func sortRows(rows []*Row, keys []SortKey) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		for _, k := range keys {
			if decided, aFirst := lessByKey(a, b, k); decided {
				return aFirst
			}
		}
		return tieBreak(a, b)
	})
}

// lessByKey compares a and b on one sort key. It returns whether the comparison
// is decisive and, if so, whether a sorts before b. Availability precedence
// (available before unavailable) is applied BEFORE direction, so unavailable
// values always sort last in both ascending and descending order.
func lessByKey(a, b *Row, k SortKey) (decided, aFirst bool) {
	an, aOK, as, aStr := fieldValue(a, k.Field)
	bn, bOK, bs, bStr := fieldValue(b, k.Field)

	if aStr || bStr {
		c := cmpString(as, bs)
		if c == 0 {
			return false, false
		}
		if k.Descending {
			return true, c > 0
		}
		return true, c < 0
	}

	switch {
	case !aOK && !bOK:
		return false, false
	case aOK && !bOK:
		return true, true // available first
	case !aOK && bOK:
		return true, false
	}
	if an == bn {
		return false, false
	}
	if k.Descending {
		return true, an > bn
	}
	return true, an < bn
}

// fieldValue returns (numeric, numericOK, stringVal, isString) for a field.
func fieldValue(r *Row, field string) (float64, bool, string, bool) {
	switch field {
	case "target", "label", "comm", "name":
		return 0, false, r.Label, true
	case "procs":
		return float64(r.Procs), true, "", false
	case "threads":
		return float64(r.Threads), true, "", false
	case "children":
		return float64(r.Children), true, "", false
	case "leaves":
		return float64(r.Leaves), true, "", false
	case "pid":
		if r.Process != nil {
			return float64(r.Process.PID), true, "", false
		}
		return 0, false, "", false
	}
	v := r.Metrics[model.MetricID(field)]
	if val, ok := v.Get(); ok {
		return val, true, "", false
	}
	return 0, false, "", false
}

func cmpString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func tieBreak(a, b *Row) bool {
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.Label != b.Label {
		return a.Label < b.Label
	}
	return a.Key < b.Key
}
