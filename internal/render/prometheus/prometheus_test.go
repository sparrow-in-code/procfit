package prometheus

import (
	"bytes"
	"strings"
	"testing"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

func TestRender(t *testing.T) {
	reg := metrics.NewDefault()
	row := &query.Row{Kind: query.RowProcess, Label: `chr"ome`, Process: &model.Process{PID: 42}}
	row.Metrics = map[model.MetricID]model.MetricValue{
		"sock-tcp": model.NewValue(5.0, model.Sampled, "socket"),
		"cpu":      model.Unavailable[float64](model.Unsupported, "x"), // unavailable → whole family omitted
	}
	res := &query.Result{
		Rows: []*query.Row{row},
		HostMetrics: map[model.MetricID]model.MetricValue{
			"cstate-deep-residency": model.NewValue(80.0, model.Sampled, "cstate"),
		},
	}
	var b bytes.Buffer
	if err := Render(&b, res, reg); err != nil {
		t.Fatal(err)
	}
	out := b.String()

	if !strings.Contains(out, "# TYPE procfit_sock_tcp gauge") {
		t.Fatalf("missing sock-tcp family:\n%s", out)
	}
	// id sanitised (- → _), label value escaped, pid label present.
	if !strings.Contains(out, `procfit_sock_tcp{target="chr\"ome",pid="42"} 5`) {
		t.Fatalf("series line wrong:\n%s", out)
	}
	// A metric with no available samples emits no family at all (no bare header).
	if strings.Contains(out, "procfit_cpu") {
		t.Fatalf("unavailable metric must be omitted, not a fabricated zero:\n%s", out)
	}
	// Host-scoped metric is an unlabelled series.
	if !strings.Contains(out, "procfit_cstate_deep_residency 80") {
		t.Fatalf("host metric family missing:\n%s", out)
	}
	// HELP carries the descriptor's description.
	if !strings.Contains(out, "# HELP procfit_sock_tcp TCP sockets held by the process") {
		t.Fatalf("HELP text missing:\n%s", out)
	}
}
