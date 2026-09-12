package query

import (
	"testing"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
)

func procWith(id model.MetricID, vals ...float64) []*model.Process {
	out := make([]*model.Process, 0, len(vals))
	for i, v := range vals {
		p := &model.Process{ID: model.ProcessInstanceID{PID: i + 1}}
		p.SetMetric(id, model.NewValue(v, model.Exact, "t"))
		out = append(out, p)
	}
	return out
}

func TestAggregateOne_Reducers(t *testing.T) {
	procs := procWith("m", 3, 1, 2)
	if v, _ := aggregateOne(metrics.AggSum, "m", procs).Get(); v != 6 {
		t.Fatalf("sum = %v, want 6", v)
	}
	if v, _ := aggregateOne(metrics.AggMin, "m", procs).Get(); v != 1 {
		t.Fatalf("min = %v, want 1", v)
	}
	if v, _ := aggregateOne(metrics.AggMax, "m", procs).Get(); v != 3 {
		t.Fatalf("max = %v, want 3", v)
	}
	// Mixed with differing values => mixed quality.
	if q := aggregateOne(metrics.AggMixed, "m", procs).Quality; q != "mixed" {
		t.Fatalf("mixed quality = %q, want mixed", q)
	}
	// Mixed uniform => the single value, available/derived.
	uni := procWith("m", 4, 4, 4)
	mv := aggregateOne(metrics.AggMixed, "m", uni)
	if v, ok := mv.Get(); !ok || v != 4 || mv.Quality == "mixed" {
		t.Fatalf("uniform mixed wrong: v=%v ok=%v q=%q", v, ok, mv.Quality)
	}
	// No present values => unavailable.
	if aggregateOne(metrics.AggSum, "m", nil).Present() {
		t.Fatal("empty aggregate must be unavailable")
	}
}
