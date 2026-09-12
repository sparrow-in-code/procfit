package csv

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

func sampleResult() *query.Result {
	leaf := &query.Row{Kind: query.RowProcess, Label: "worker", Procs: 1, Threads: 2, Process: &model.Process{PID: 7}}
	leaf.Metrics = map[model.MetricID]model.MetricValue{
		"cpu": model.NewValue(2.5, model.Derived, "t"),
		"rss": model.Unavailable[float64](model.PermissionDenied, "t"),
	}
	g := &query.Row{Kind: query.RowGroup, Label: "grp", Procs: 1, Threads: 2, Sub: []*query.Row{leaf}}
	g.Metrics = map[model.MetricID]model.MetricValue{"cpu": model.NewValue(2.5, model.Derived, "a")}
	return &query.Result{Generation: 1, WallTime: time.Unix(0, 0).UTC(), Rows: []*query.Row{g}}
}

func TestCSV_RawValuesAndUnavailableEmpty(t *testing.T) {
	reg := metrics.NewDefault()
	cols, _ := render.ResolveColumns(reg, []string{"target", "cpu", "rss"})
	var buf bytes.Buffer
	if err := Render(&buf, sampleResult(), cols); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if lines[0] != "time,depth,kind,target,cpu,rss" {
		t.Fatalf("header wrong: %q", lines[0])
	}
	// group row depth 0, leaf depth 1; cpu raw 2.5 (not "2.5" formatted with unit); rss empty.
	if !strings.Contains(buf.String(), ",2.5,") {
		t.Fatalf("expected raw cpu 2.5:\n%s", buf.String())
	}
	last := lines[len(lines)-1]
	fields := strings.Split(last, ",")
	if fields[len(fields)-1] != "" {
		t.Fatalf("unavailable rss should be an empty field, got %q in %q", fields[len(fields)-1], last)
	}
}

func TestCSV_HeaderAndRowsSeparate(t *testing.T) {
	reg := metrics.NewDefault()
	cols, _ := render.ResolveColumns(reg, []string{"target", "cpu"})
	var h, r bytes.Buffer
	if err := WriteHeader(&h, cols); err != nil {
		t.Fatal(err)
	}
	if err := WriteRows(&r, sampleResult(), cols); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h.String(), "time,depth,kind,target,cpu") {
		t.Fatalf("header: %q", h.String())
	}
	if strings.Contains(r.String(), "target,cpu") {
		t.Fatal("rows output must not contain the header")
	}
}
