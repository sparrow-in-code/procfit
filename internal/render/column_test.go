package render

import (
	"testing"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

func TestResolveColumns_StructuralAndMetric(t *testing.T) {
	reg := metrics.NewDefault()
	cols, err := ResolveColumns(reg, []string{"target", "pt", "procs", "threads", "pid", "ppid", "comm", "uid", "pstate", "kind", "cpu"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 11 {
		t.Fatalf("want 11 columns, got %d", len(cols))
	}
	if !cols[0].IsTarget {
		t.Fatal("target column should be marked IsTarget")
	}

	proc := &model.Process{PID: 42, PPID: 7, UID: 1000, Comm: "bash", State: model.ProcessState{Code: model.StateSleeping}}
	row := &query.Row{Kind: query.RowProcess, Label: "bash", Procs: 1, Threads: 3, Process: proc}
	row.Metrics = map[model.MetricID]model.MetricValue{"cpu": model.NewValue(2.5, model.Derived, "t")}

	want := map[string]string{
		"target": "bash", "pt": "1/3", "procs": "1", "threads": "3",
		"pid": "42", "ppid": "7", "comm": "bash", "uid": "1000", "pstate": "S", "kind": "process", "cpu": "2.5",
	}
	for _, c := range cols {
		if got := c.Cell(row); got != want[c.ID] {
			t.Errorf("column %s cell = %q, want %q", c.ID, got, want[c.ID])
		}
	}
}

func TestResolveColumns_Unknown(t *testing.T) {
	if _, err := ResolveColumns(metrics.NewDefault(), []string{"nope"}); err == nil {
		t.Fatal("unknown column should error")
	}
}

func TestStructuralColumns_GroupRows(t *testing.T) {
	cols, _ := ResolveColumns(metrics.NewDefault(), []string{"pid", "ppid", "comm", "uid", "pstate"})
	group := &query.Row{Kind: query.RowGroup, Label: "chrome"}
	for _, c := range cols {
		if got := c.Cell(group); got != "" {
			t.Errorf("group row column %s should be empty, got %q", c.ID, got)
		}
	}
}
