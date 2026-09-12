package query

import (
	"testing"
	"time"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
)

func proc(pid int, comm string, cpu, rss float64, threads int) model.Process {
	p := model.Process{
		ID:         model.ProcessInstanceID{PID: pid, StartTime: uint64(pid)},
		PID:        pid,
		Comm:       comm,
		NumThreads: threads,
	}
	p.SetMetric("cpu", model.NewValue(cpu, model.Derived, "t"))
	p.SetMetric("rss", model.NewValue(rss, model.Exact, "t"))
	return p
}

func newEngine() *Engine { return NewEngine(metrics.NewDefault(), NewDimensions()) }

func input(ps ...model.Process) Input {
	return Input{Generation: 1, WallTime: time.Unix(0, 0), Processes: ps}
}

func TestEngine_FlatProcessList(t *testing.T) {
	e := newEngine()
	res, err := e.Build(input(proc(1, "a", 1, 100, 1), proc(2, "b", 2, 200, 1)),
		QuerySpec{Metrics: []model.MetricID{"cpu", "rss"}, GroupBy: []string{"none"}, Leaf: LeafProcess})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("want 2 top-level process rows, got %d", len(res.Rows))
	}
	for _, r := range res.Rows {
		if r.Kind != RowProcess {
			t.Fatalf("want process rows, got %s", r.Kind)
		}
	}
}

func TestEngine_GroupByCommAggregatesCPU(t *testing.T) {
	e := newEngine()
	// Two chrome procs + one idea.
	res, err := e.Build(
		input(proc(1, "chrome", 1.5, 1000, 2), proc(2, "chrome", 2.0, 2000, 3), proc(3, "idea", 4.0, 500, 1)),
		QuerySpec{Metrics: []model.MetricID{"cpu", "rss"}, GroupBy: []string{"comm"}, Leaf: LeafNone, Sort: []SortKey{{Field: "cpu", Descending: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("want 2 groups, got %d", len(res.Rows))
	}
	// idea (4.0) should sort before chrome (3.5) descending.
	if res.Rows[0].Label != "idea" {
		t.Fatalf("sort desc by cpu wrong: first is %q", res.Rows[0].Label)
	}
	chrome := res.Rows[1]
	if chrome.Label != "chrome" {
		t.Fatalf("second group %q", chrome.Label)
	}
	cpu, _ := chrome.Metrics["cpu"].Get()
	if cpu < 3.49 || cpu > 3.51 {
		t.Fatalf("chrome aggregate cpu = %v, want 3.5", cpu)
	}
	if chrome.Procs != 2 {
		t.Fatalf("chrome procs = %d, want 2", chrome.Procs)
	}
	if chrome.Threads != 5 {
		t.Fatalf("chrome threads = %d, want 5", chrome.Threads)
	}
	// rss summed once per process: 1000+2000 = 3000
	rss, _ := chrome.Metrics["rss"].Get()
	if rss != 3000 {
		t.Fatalf("chrome rss = %v, want 3000", rss)
	}
}

func TestEngine_HostTotal(t *testing.T) {
	e := newEngine()
	res, err := e.Build(input(proc(1, "a", 1, 100, 1), proc(2, "b", 2, 200, 2)),
		QuerySpec{Metrics: []model.MetricID{"cpu"}, GroupBy: []string{"none"}, Leaf: LeafNone})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || res.Rows[0].Kind != RowHost {
		t.Fatal("group none + leaf none should yield one host-total row")
	}
	cpu, _ := res.Rows[0].Metrics["cpu"].Get()
	if cpu != 3 {
		t.Fatalf("host cpu = %v, want 3", cpu)
	}
	if res.Rows[0].Procs != 2 || res.Rows[0].Threads != 3 {
		t.Fatalf("host counts wrong: procs=%d threads=%d", res.Rows[0].Procs, res.Rows[0].Threads)
	}
}

func TestEngine_GroupWithProcessLeaves(t *testing.T) {
	e := newEngine()
	res, err := e.Build(
		input(proc(1, "chrome", 1, 100, 1), proc(2, "chrome", 2, 200, 1)),
		QuerySpec{Metrics: []model.MetricID{"cpu"}, GroupBy: []string{"comm"}, Leaf: LeafProcess})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("want 1 group, got %d", len(res.Rows))
	}
	g := res.Rows[0]
	if g.Children != 2 || g.Leaves != 2 {
		t.Fatalf("group children=%d leaves=%d, want 2/2", g.Children, g.Leaves)
	}
	if len(g.Sub) != 2 || g.Sub[0].Kind != RowProcess {
		t.Fatal("group should have 2 process leaf children")
	}
}

func TestEngine_GroupByValidation(t *testing.T) {
	e := newEngine()
	cases := [][]string{{"none", "comm"}, {"comm", "comm"}, {"bogus"}}
	for _, gb := range cases {
		if _, err := e.Build(input(proc(1, "a", 1, 1, 1)), QuerySpec{GroupBy: gb, Leaf: LeafNone}); err == nil {
			t.Fatalf("group-by %v should be rejected", gb)
		}
	}
}

func TestEngine_UnavailableSortsLast(t *testing.T) {
	e := newEngine()
	p1 := proc(1, "a", 5, 100, 1)
	p2 := proc(2, "b", 1, 100, 1)
	// p3 has no cpu value (unavailable).
	p3 := model.Process{ID: model.ProcessInstanceID{PID: 3, StartTime: 3}, PID: 3, Comm: "c", NumThreads: 1}
	p3.SetMetric("rss", model.NewValue(1.0, model.Exact, "t"))
	res, _ := e.Build(input(p1, p2, p3),
		QuerySpec{Metrics: []model.MetricID{"cpu"}, GroupBy: []string{"none"}, Leaf: LeafProcess, Sort: []SortKey{{Field: "cpu", Descending: true}}})
	last := res.Rows[len(res.Rows)-1]
	if last.Label != "c" {
		t.Fatalf("unavailable cpu should sort last, got last=%q", last.Label)
	}
}
