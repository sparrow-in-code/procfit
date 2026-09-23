package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/testutil"
)

func hostAsm(t *testing.T) *assembly {
	t.Helper()
	return newAssemblyWith(testutil.NewFakeSource(nil), testutil.NewFakeClock(time.Unix(1000, 0)))
}

func TestLayoutMatrix_ColumnMajorAligned(t *testing.T) {
	// 4 cells over 3 rows → 2 columns, filled top-bottom then left-right:
	// col0 = a,bb,ccc ; col1 = d (row0 only).
	got := layoutMatrix([]string{"a=1", "bb=22", "ccc=333", "d=4"}, 3)
	if len(got) != 3 {
		t.Fatalf("want 3 rows, got %d: %q", len(got), got)
	}
	if !strings.HasPrefix(got[0], "a=1") || !strings.HasSuffix(got[0], "d=4") {
		t.Fatalf("row0 layout wrong: %q", got[0])
	}
	if got[1] != "bb=22" || got[2] != "ccc=333" {
		t.Fatalf("rows 1/2 wrong: %q", got)
	}
	// col0 padded to the widest cell (ccc=333, 7) + 2 → d=4 starts at index 9.
	if idx := strings.Index(got[0], "d=4"); idx != 9 {
		t.Fatalf("column not aligned: d=4 at %d, want 9 (%q)", idx, got[0])
	}
	if layoutMatrix(nil, 3) != nil {
		t.Fatal("no cells → no rows")
	}
}

func TestQuoteIfSpace(t *testing.T) {
	if quoteIfSpace("plain") != "plain" {
		t.Fatal("no-space value must not be quoted")
	}
	if got := quoteIfSpace("has space"); got != `"has space"` {
		t.Fatalf("space value must be quoted, got %q", got)
	}
}

func TestProcessColumns_DropsHostMetrics(t *testing.T) {
	a := hostAsm(t)
	got := a.processColumns([]string{"target", "pid", "cpu", "power-pkg", "cstate-deep-residency", "psi-cpu"})
	want := []string{"target", "pid", "cpu"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("processColumns = %v, want %v (host metrics dropped)", got, want)
	}
}

func TestHostSection(t *testing.T) {
	a := hostAsm(t)
	res := &query.Result{
		WallTime: time.Unix(1_700_000_000, 0),
		Rows:     []*query.Row{{Procs: 2}},
		HostMetrics: map[model.MetricID]model.MetricValue{
			"psi-cpu":               model.NewValue(1.5, model.Sampled, "psi"),
			"cstate-deep-residency": model.NewValue(80.0, model.Sampled, "cstate"),
		},
	}
	sec := a.hostSection(res, false)
	if len(sec) < 3 {
		t.Fatalf("expected banner + blank + matrix, got %q", sec)
	}
	if !strings.Contains(sec[0], "2 processes") {
		t.Fatalf("banner should report process count: %q", sec[0])
	}
	if sec[1] != "" {
		t.Fatalf("banner and matrix must be blank-line separated, got %q", sec[1])
	}
	body := strings.Join(sec[2:], "\n")
	if !strings.Contains(body, "psi-cpu=1.5") || !strings.Contains(body, "cstate-deep-residency=80.0") {
		t.Fatalf("matrix missing formatted host values:\n%s", body)
	}
	// No host metrics → no section at all (output unchanged for plain ps).
	if a.hostSection(&query.Result{}, false) != nil {
		t.Fatal("empty host metrics must yield no section")
	}
}

type fakeHostColl struct {
	id  model.MetricID
	val model.MetricValue
}

func (f fakeHostColl) ID() string                { return "fake" }
func (f fakeHostColl) Metrics() []model.MetricID { return []model.MetricID{f.id} }
func (f fakeHostColl) CollectHost(context.Context) map[model.MetricID]model.MetricValue {
	return map[model.MetricID]model.MetricValue{f.id: f.val}
}

func TestCollectHost_OnlyRunsRequested(t *testing.T) {
	a := hostAsm(t)
	a.hostCollectors = []ports.HostCollector{
		fakeHostColl{id: "psi-cpu", val: model.NewValue(3.0, model.Sampled, "fake")},
		fakeHostColl{id: "power-pkg", val: model.NewValue(9.0, model.Sampled, "fake")},
	}
	got := a.collectHost(context.Background(), []model.MetricID{"psi-cpu"})
	if len(got) != 1 {
		t.Fatalf("only requested host metric should be collected: %v", got)
	}
	if v := got["psi-cpu"]; !v.Present() || v.V != 3 {
		t.Fatalf("psi-cpu = %+v, want 3", v)
	}
}
