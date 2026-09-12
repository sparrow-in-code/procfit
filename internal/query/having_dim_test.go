package query

import (
	"testing"

	"github.com/netikras/procfit/internal/model"
)

// cpuAtLeast is a RowPredicate keeping rows whose cpu >= threshold.
type cpuAtLeast struct{ min float64 }

func (c cpuAtLeast) EvalRow(r *Row) bool {
	v, ok := r.Metrics["cpu"].Get()
	return ok && v >= c.min
}

// uidPred is an EntityPredicate matching a uid.
type uidPred struct{ uid uint32 }

func (u uidPred) EvalEntity(p *model.Process) bool { return p.UID == u.uid }

func TestEngine_HavingFiltersGroups(t *testing.T) {
	e := newEngine()
	res, err := e.Build(input(proc(1, "a", 10, 100, 1), proc(2, "b", 1, 100, 1)),
		QuerySpec{Metrics: []model.MetricID{"cpu"}, GroupBy: []string{"comm"}, Leaf: LeafNone, Having: cpuAtLeast{min: 5}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || res.Rows[0].Label != "a" {
		t.Fatalf("having should keep only group a, got %d rows", len(res.Rows))
	}
}

func TestEngine_SelectFiltersEntities(t *testing.T) {
	e := newEngine()
	p1 := proc(1, "a", 1, 1, 1)
	p1.UID = 1000
	p2 := proc(2, "b", 1, 1, 1)
	p2.UID = 0
	res, _ := e.Build(input(p1, p2),
		QuerySpec{Metrics: []model.MetricID{"cpu"}, GroupBy: []string{"none"}, Leaf: LeafProcess, Select: uidPred{uid: 1000}})
	if len(res.Rows) != 1 || res.Rows[0].Label != "a" {
		t.Fatalf("select should keep only uid 1000, got %d rows", len(res.Rows))
	}
}

func TestEngine_UnknownBucket(t *testing.T) {
	e := newEngine()
	// A process with no app id groups under an "unknown" bucket for dimension app.
	p := proc(1, "a", 1, 1, 1)
	res, err := e.Build(input(p), QuerySpec{Metrics: []model.MetricID{"cpu"}, GroupBy: []string{"app"}, Leaf: LeafNone})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("want 1 group, got %d", len(res.Rows))
	}
	if res.Rows[0].Label != "app:?" {
		t.Fatalf("unknown app should render 'app:?', got %q", res.Rows[0].Label)
	}
}

func TestEngine_ThreadLeaves(t *testing.T) {
	e := newEngine()
	p := proc(1, "a", 1, 1, 2)
	p.Threads = []model.Thread{
		{ID: model.ThreadInstanceID{TID: 11}, Comm: "t1"},
		{ID: model.ThreadInstanceID{TID: 12}, Comm: "t2"},
	}
	res, _ := e.Build(input(p), QuerySpec{Metrics: []model.MetricID{"cpu"}, GroupBy: []string{"comm"}, Leaf: LeafThread})
	if len(res.Rows) != 1 {
		t.Fatalf("want 1 group")
	}
	if res.Rows[0].Leaves != 2 {
		t.Fatalf("want 2 thread leaves, got %d", res.Rows[0].Leaves)
	}
	if res.Rows[0].Sub[0].Kind != RowThread {
		t.Fatalf("leaf should be a thread row")
	}
}

func TestEngine_NamespaceGrouping(t *testing.T) {
	e := newEngine()
	p1 := proc(1, "a", 1, 1, 1)
	p1.Namespaces = model.NamespaceSet{model.NSPID: 4026531836}
	p2 := proc(2, "b", 1, 1, 1)
	p2.Namespaces = model.NamespaceSet{model.NSPID: 4026531836}
	p3 := proc(3, "c", 1, 1, 1)
	p3.Namespaces = model.NamespaceSet{model.NSPID: 4026999999}
	res, err := e.Build(input(p1, p2, p3), QuerySpec{Metrics: []model.MetricID{"cpu"}, GroupBy: []string{"pidns"}, Leaf: LeafNone})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("want 2 pidns groups, got %d", len(res.Rows))
	}
}
