package queryspec

import (
	"testing"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

func regDims() (*metrics.Registry, *query.Dimensions) {
	return metrics.NewDefault(), query.NewDimensions()
}

func TestBuild_Defaults(t *testing.T) {
	reg, dims := regDims()
	r, err := Build(reg, dims, Flags{GroupBy: "comm", Leaf: "none", Sort: "cpu:desc"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Spec.Leaf != query.LeafNone || len(r.Spec.GroupBy) != 1 || r.Spec.GroupBy[0] != "comm" {
		t.Fatalf("spec wrong: %+v", r.Spec)
	}
	if len(r.Spec.Sort) != 1 || !r.Spec.Sort[0].Descending {
		t.Fatalf("sort wrong: %+v", r.Spec.Sort)
	}
	if len(r.Columns) == 0 {
		t.Fatal("columns should default")
	}
	// needed metrics include cpu (from light profile + sort).
	found := false
	for _, id := range r.Needed {
		if id == model.MetricID("cpu") {
			found = true
		}
	}
	if !found {
		t.Fatal("needed should include cpu")
	}
}

func TestBuild_WideColumns(t *testing.T) {
	reg, dims := regDims()
	r, _ := Build(reg, dims, Flags{Format: "wide"})
	if len(r.Columns) != len(WideColumns) {
		t.Fatalf("wide should use wide columns, got %d", len(r.Columns))
	}
}

func TestBuild_SelectHavingValidated(t *testing.T) {
	reg, dims := regDims()
	if _, err := Build(reg, dims, Flags{Select: "uid == 0"}); err != nil {
		t.Fatalf("valid select rejected: %v", err)
	}
	if _, err := Build(reg, dims, Flags{Select: "bogusfield == 1"}); err == nil {
		t.Fatal("invalid select field should error")
	}
	if _, err := Build(reg, dims, Flags{Having: "cpu > 1"}); err != nil {
		t.Fatalf("valid having rejected: %v", err)
	}
	if _, err := Build(reg, dims, Flags{Sort: "cpu:sideways"}); err == nil {
		t.Fatal("bad sort direction should error")
	}
	if _, err := Build(reg, dims, Flags{MetricOverrides: []string{"+bogus"}}); err == nil {
		t.Fatal("unknown metric should error")
	}
}

func TestParseOverrides(t *testing.T) {
	got, _ := ParseOverrides([]string{"cpu", "+rss", "-vsz", ""})
	if len(got) != 3 || !got[0].Add || got[0].ID != "cpu" || got[2].Add {
		t.Fatalf("override parse wrong: %+v", got)
	}
}

func TestSplitComma(t *testing.T) {
	if got := SplitComma(" a, b ,,c "); len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("SplitComma wrong: %v", got)
	}
	if SplitComma("") != nil {
		t.Fatal("empty should be nil")
	}
}

func TestEnvAdapters(t *testing.T) {
	p := &model.Process{PID: 5, UID: 1000, Comm: "x", User: "alice"}
	p.SetMetric("cpu", model.NewValue(3.0, model.Derived, "t"))
	e := EntityEnv{P: p}
	if v := e.Lookup("cpu"); v.Kind != 1 /*number*/ || v.Num != 3 {
		t.Fatalf("cpu lookup wrong: %+v", v)
	}
	if v := e.Lookup("user"); v.Str != "alice" {
		t.Fatalf("user lookup wrong: %+v", v)
	}
	if v := e.Lookup("nope"); !v.IsMissing() {
		t.Fatal("unknown field should be missing")
	}
	row := &query.Row{Kind: query.RowGroup, Label: "g", Procs: 2}
	row.Metrics = map[model.MetricID]model.MetricValue{"cpu": model.NewValue(9.0, model.Derived, "a")}
	re := RowEnv{R: row}
	if v := re.Lookup("procs"); v.Num != 2 {
		t.Fatal("procs lookup wrong")
	}
	if v := re.Lookup("cpu"); v.Num != 9 {
		t.Fatal("row cpu lookup wrong")
	}
}
