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

func TestChooseColumns_LeafIdentifiers(t *testing.T) {
	reg, dims := regDims()
	// Process-leaf default exposes pid as its own column, but not tid.
	r, _ := Build(reg, dims, Flags{Leaf: "process"})
	if !containsCol(r.Columns, "pid") || containsCol(r.Columns, "tid") {
		t.Fatalf("process default should have pid and no tid: %v", r.Columns)
	}
	// Thread-leaf default exposes both pid and tid.
	rt, _ := Build(reg, dims, Flags{Leaf: "thread"})
	if !containsCol(rt.Columns, "pid") || !containsCol(rt.Columns, "tid") {
		t.Fatalf("thread default should have pid and tid: %v", rt.Columns)
	}
	// Explicit --columns is respected verbatim (no identifier injection).
	rc, _ := Build(reg, dims, Flags{Leaf: "thread", Columns: "target,cpu"})
	if containsCol(rc.Columns, "pid") || containsCol(rc.Columns, "tid") {
		t.Fatalf("explicit columns must be verbatim: %v", rc.Columns)
	}
}

func containsCol(cols []string, id string) bool {
	for _, c := range cols {
		if c == id {
			return true
		}
	}
	return false
}

func TestChooseColumns_ProfileDriven(t *testing.T) {
	reg, dims := regDims()
	// An explicit profile drives the default display: identity + its metrics.
	r, _ := Build(reg, dims, Flags{Profile: "io"})
	if r.Columns[0] != "target" || !containsCol(r.Columns, "pid") || !containsCol(r.Columns, "disk-rbps") {
		t.Fatalf("io profile should drive columns (target,pid,+io metrics): %v", r.Columns)
	}
	if containsCol(r.Columns, "pstate") {
		t.Fatalf("profile columns should be identity+metrics, not the compact defaults: %v", r.Columns)
	}
	// Explicit --columns still overrides the profile.
	rc, _ := Build(reg, dims, Flags{Profile: "io", Columns: "target,cpu"})
	if len(rc.Columns) != 2 {
		t.Fatalf("explicit --columns must override the profile: %v", rc.Columns)
	}
	// No profile (default light) keeps the compact defaults.
	rd, _ := Build(reg, dims, Flags{})
	if !containsCol(rd.Columns, "pt") {
		t.Fatalf("default should keep compact columns: %v", rd.Columns)
	}
}

func TestBuild_ProfileViewDefaults(t *testing.T) {
	reg, dims := regDims()
	// sysload predefines explicit columns (incl pstate/wchan) + sort runq-delay:desc.
	r, err := Build(reg, dims, Flags{Profile: "sysload"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Columns[0] != "target" || !containsCol(r.Columns, "pstate") || !containsCol(r.Columns, "wchan") {
		t.Fatalf("sysload should use its explicit columns incl pstate/wchan: %v", r.Columns)
	}
	if len(r.Spec.Sort) == 0 || r.Spec.Sort[0].Field != "runq-delay" {
		t.Fatalf("sysload should default-sort by runq-delay: %+v", r.Spec.Sort)
	}
	// Explicit --sort wins over the profile default.
	ro, _ := Build(reg, dims, Flags{Profile: "sysload", Sort: "cpu:desc"})
	if len(ro.Spec.Sort) == 0 || ro.Spec.Sort[0].Field != "cpu" {
		t.Fatalf("explicit --sort must override the profile default: %+v", ro.Spec.Sort)
	}
}

func TestBuild_ProfilesAlwaysShowGroupSize(t *testing.T) {
	reg, dims := regDims()
	ptAfterTarget := func(cols []string) bool {
		for i, c := range cols {
			if c == "pt" {
				return i > 0 && cols[i-1] == "target"
			}
		}
		return false
	}
	// Explicit-column profile (sysload) and derived profile (io) both get pt,
	// right after target, so group size is always visible.
	for _, p := range []string{"sysload", "io"} {
		r, _ := Build(reg, dims, Flags{Profile: p})
		if !containsCol(r.Columns, "pt") || !ptAfterTarget(r.Columns) {
			t.Fatalf("profile %q should include pt after target: %v", p, r.Columns)
		}
	}
	// Explicit --columns is respected — pt is NOT forced in.
	rc, _ := Build(reg, dims, Flags{Columns: "target,cpu"})
	if containsCol(rc.Columns, "pt") {
		t.Fatalf("explicit --columns must not gain pt: %v", rc.Columns)
	}
}

func TestBuild_StateWchanFilters(t *testing.T) {
	reg, dims := regDims()
	if _, err := Build(reg, dims, Flags{Select: `state == "D"`}); err != nil {
		t.Fatalf("select by state rejected: %v", err)
	}
	// wchan is valid in having and surfaces in ExprFields so the app can enable
	// the optional /proc/PID/wchan read for the query.
	r, err := Build(reg, dims, Flags{Having: `wchan ~= "jbd2"`})
	if err != nil {
		t.Fatalf("having by wchan rejected: %v", err)
	}
	if !containsCol(r.ExprFields, "wchan") {
		t.Fatalf("wchan filter should surface in ExprFields, got %v", r.ExprFields)
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

func TestBuild_HavingSortColumnsNeeded(t *testing.T) {
	reg, dims := regDims()
	r, err := Build(reg, dims, Flags{
		GroupBy: "comm", Leaf: "none", Having: "cpu > 1",
		Columns: "target,rss", Sort: "disk-rbps:desc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Spec.Having == nil {
		t.Fatal("having predicate should be set")
	}
	// needed must include cpu (having), rss (column), disk-rbps (sort).
	want := map[string]bool{"cpu": true, "rss": true, "disk-rbps": true}
	for _, id := range r.Needed {
		delete(want, string(id))
	}
	if len(want) != 0 {
		t.Fatalf("needed metrics missing %v (got %v)", want, r.Needed)
	}
}

func TestBuild_HavingInvalidField(t *testing.T) {
	reg, dims := regDims()
	if _, err := Build(reg, dims, Flags{Having: "bogusrow > 1"}); err == nil {
		t.Fatal("invalid having field should error")
	}
}

func TestParseOverrides(t *testing.T) {
	got, _ := ParseOverrides([]string{"cpu", "+rss", "-vsz", ""})
	if len(got) != 3 || !got[0].Add || got[0].ID != "cpu" || got[2].Add {
		t.Fatalf("override parse wrong: %+v", got)
	}
}

func TestPredicatesEval(t *testing.T) {
	reg, dims := regDims()
	r, err := Build(reg, dims, Flags{Select: "uid == 0", Having: "procs > 1"})
	if err != nil {
		t.Fatal(err)
	}
	root := &model.Process{UID: 0}
	nonroot := &model.Process{UID: 1000}
	if !r.Spec.Select.EvalEntity(root) || r.Spec.Select.EvalEntity(nonroot) {
		t.Fatal("select uid==0 predicate wrong")
	}
	big := &query.Row{Procs: 2}
	small := &query.Row{Procs: 1}
	if !r.Spec.Having.EvalRow(big) || r.Spec.Having.EvalRow(small) {
		t.Fatal("having procs>1 predicate wrong")
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
	// name is a displayed, Filterable column, so it must be resolvable (and
	// allowed) in a `having` expression — mapping to the row label like comm.
	if v := re.Lookup("name"); v.Str != "g" {
		t.Fatalf("row name should resolve to the label, got %+v", v)
	}
	if !AllowedRowFields(metrics.NewRegistry())["name"] {
		t.Fatal("name must be an allowed having field")
	}
}
