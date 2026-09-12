package table

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/render"
)

func TestRender_TreeAndUnavailable(t *testing.T) {
	reg := metrics.NewDefault()
	// One group with one process leaf; cpu unavailable on the process.
	leaf := &query.Row{Kind: query.RowProcess, Label: "chrome", Procs: 1, Threads: 4,
		Process: &model.Process{PID: 42, State: model.ProcessState{Code: model.StateSleeping}}}
	leaf.Metrics = map[model.MetricID]model.MetricValue{
		"cpu": model.Unavailable[float64](model.WarmingUp, "proc"),
		"rss": model.NewValue(4096.0, model.Exact, "proc"),
	}
	group := &query.Row{Kind: query.RowGroup, Label: "chrome", Procs: 1, Threads: 4, Children: 1, Leaves: 1, Sub: []*query.Row{leaf}}
	group.Metrics = map[model.MetricID]model.MetricValue{
		"cpu": model.NewValue(2.5, model.Derived, "agg"),
		"rss": model.NewValue(4096.0, model.Derived, "agg"),
	}
	res := &query.Result{Generation: 1, WallTime: time.Unix(0, 0), Rows: []*query.Row{group}}

	cols, err := render.ResolveColumns(reg, []string{"target", "pt", "cpu", "rss", "pstate"})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Render(&buf, res, cols, 0); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "TARGET") || !strings.Contains(out, "P/T") {
		t.Fatalf("header missing:\n%s", out)
	}
	// process leaf is indented under the group
	if !strings.Contains(out, "  chrome") {
		t.Fatalf("expected indented leaf:\n%s", out)
	}
	// unavailable cpu on the leaf renders as '-', not 0
	lines := strings.Split(strings.TrimSpace(out), "\n")
	leafLine := lines[len(lines)-1]
	if strings.Contains(leafLine, "0.0") {
		t.Fatalf("unavailable cpu must not render as zero: %q", leafLine)
	}
	if !strings.Contains(leafLine, "-") {
		t.Fatalf("unavailable cpu should render as '-': %q", leafLine)
	}
}

func TestRender_TargetWidthCap(t *testing.T) {
	reg := metrics.NewDefault()
	long := &query.Row{Kind: query.RowProcess, Label: "a-very-long-process-name-that-exceeds", Process: &model.Process{PID: 1}}
	res := &query.Result{Rows: []*query.Row{long}}
	cols, _ := render.ResolveColumns(reg, []string{"target", "pid"})

	var buf bytes.Buffer
	if err := Render(&buf, res, cols, 12); err != nil {
		t.Fatal(err)
	}
	// The target cell must be truncated to 12 cells with an ellipsis.
	if !strings.Contains(buf.String(), "a-very-long…") {
		t.Fatalf("target not capped to width:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "exceeds") {
		t.Fatalf("full label should not appear when capped:\n%s", buf.String())
	}
}
