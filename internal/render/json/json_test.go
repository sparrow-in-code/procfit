package json

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

func TestRender_EnvelopeAndMetrics(t *testing.T) {
	reg := metrics.NewDefault()
	leaf := &query.Row{Kind: query.RowProcess, Label: "worker", Key: "k", Procs: 1, Threads: 2,
		Process: &model.Process{PID: 99}}
	leaf.Metrics = map[model.MetricID]model.MetricValue{
		"cpu": model.NewValue(12.5, model.Derived, "proc"),
		"rss": model.Unavailable[float64](model.PermissionDenied, "proc"),
	}
	res := &query.Result{Generation: 3, WallTime: time.Unix(1_600_000_000, 0).UTC(), Elapsed: time.Second, Rows: []*query.Row{leaf}}

	var buf bytes.Buffer
	if err := Render(&buf, res, reg); err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("output is not valid json: %v\n%s", err, buf.String())
	}
	if env["schema_version"].(float64) != 1 {
		t.Fatalf("schema_version wrong: %v", env["schema_version"])
	}
	rows := env["rows"].([]any)
	row0 := rows[0].(map[string]any)
	if row0["pid"].(float64) != 99 {
		t.Fatalf("pid missing/incorrect: %v", row0["pid"])
	}
	m := row0["metrics"].(map[string]any)
	cpu := m["cpu"].(map[string]any)
	if cpu["value"].(float64) != 12.5 || cpu["availability"] != "available" {
		t.Fatalf("cpu metric wrong: %v", cpu)
	}
	rss := m["rss"].(map[string]any)
	if rss["value"] != nil {
		t.Fatalf("unavailable rss must have null value, got %v", rss["value"])
	}
	if rss["availability"] != "permission_denied" {
		t.Fatalf("rss availability wrong: %v", rss["availability"])
	}
}

func TestRender_NestedGroups(t *testing.T) {
	reg := metrics.NewDefault()
	child := &query.Row{Kind: query.RowProcess, Label: "c", Procs: 1}
	group := &query.Row{Kind: query.RowGroup, Label: "g", Procs: 1, Sub: []*query.Row{child}}
	res := &query.Result{Rows: []*query.Row{group}}
	var buf bytes.Buffer
	if err := Render(&buf, res, reg); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"children"`)) {
		t.Fatalf("nested children not emitted:\n%s", buf.String())
	}
}
