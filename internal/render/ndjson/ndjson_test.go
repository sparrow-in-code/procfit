package ndjson

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

func result() *query.Result {
	leaf := &query.Row{Kind: query.RowProcess, Label: "worker", Procs: 1, Threads: 2, Process: &model.Process{PID: 7}}
	leaf.Metrics = map[model.MetricID]model.MetricValue{
		"cpu": model.NewValue(2.5, model.Derived, "t"),
		"rss": model.Unavailable[float64](model.PermissionDenied, "t"),
	}
	return &query.Result{Generation: 4, WallTime: time.Unix(0, 0).UTC(), Rows: []*query.Row{leaf}}
}

func TestNDJSON_MetaThenRows(t *testing.T) {
	var buf bytes.Buffer
	s := NewStreamer(&buf, metrics.NewDefault())
	if err := s.WriteResult(result()); err != nil {
		t.Fatal(err)
	}
	// A second batch should NOT repeat the meta record.
	if err := s.WriteResult(result()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 { // 1 meta + 2 row records
		t.Fatalf("want 3 lines (meta + 2 rows), got %d:\n%s", len(lines), buf.String())
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &meta); err != nil || meta["record"] != "meta" {
		t.Fatalf("first line should be meta: %v %q", err, lines[0])
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &row); err != nil {
		t.Fatal(err)
	}
	if row["record"] != "row" || row["label"] != "worker" || row["pid"].(float64) != 7 {
		t.Fatalf("row record wrong: %v", row)
	}
	m := row["metrics"].(map[string]any)
	if m["cpu"].(map[string]any)["value"].(float64) != 2.5 {
		t.Fatalf("cpu value wrong: %v", m["cpu"])
	}
	if m["rss"].(map[string]any)["value"] != nil {
		t.Fatal("unavailable rss must have null value")
	}
}
