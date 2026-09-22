package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/render"
)

func sampleResult(n int) (*query.Result, []render.Column) {
	rows := make([]*query.Row, 0, n)
	for i := 0; i < n; i++ {
		r := &query.Row{Kind: query.RowProcess, Label: label(i), Procs: 1, Threads: 1,
			Process: &model.Process{PID: i + 1}}
		r.Metrics = map[model.MetricID]model.MetricValue{"cpu": model.NewValue(float64(i), model.Derived, "t")}
		rows = append(rows, r)
	}
	cols, _ := render.ResolveColumns(metrics.NewDefault(), []string{"target", "pt", "cpu"})
	return &query.Result{Generation: 7, WallTime: time.Unix(0, 0), Rows: rows}, cols
}

func label(i int) string { return "proc" + string(rune('a'+i%26)) }

// groupedResult builds a one-group tree (group-one with two process leaves) for
// exercising the tree renderer and expand/collapse.
func groupedResult() (*query.Result, []render.Column) {
	mk := func(pid int) *query.Row {
		r := &query.Row{Kind: query.RowProcess, Key: "p" + string(rune('0'+pid)), Label: label(pid),
			Procs: 1, Threads: 1, Process: &model.Process{PID: pid}}
		r.Metrics = map[model.MetricID]model.MetricValue{"cpu": model.NewValue(float64(pid), model.Derived, "t")}
		return r
	}
	g := &query.Row{Kind: query.RowGroup, Key: "g1", Label: "group-one", Children: 2, Procs: 2, Threads: 2,
		Sub: []*query.Row{mk(1), mk(2)}}
	g.Metrics = map[model.MetricID]model.MetricValue{"cpu": model.NewValue(3.0, model.Derived, "t")}
	cols, _ := render.ResolveColumns(metrics.NewDefault(), []string{"target", "pt", "cpu"})
	return &query.Result{Generation: 7, WallTime: time.Unix(0, 0), Rows: []*query.Row{g}}, cols
}

func TestModel_FlatViewNoIndent(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 10)
	res, cols := sampleResult(3) // flat process rows, no groups
	m.SetResult(res, cols)
	if m.hasGroups {
		t.Fatal("a flat process list must not be treated as a tree")
	}
	// No groups => no fold marker/indent; the label follows the cursor gap.
	if row := m.Frame()[2]; !strings.HasPrefix(row, "> proc") {
		t.Fatalf("flat leaf must not be indented, got %q", row)
	}
}

func TestModel_ExpandCollapse(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 24)
	res, cols := groupedResult()
	m.SetResult(res, cols)

	// Expanded by default: group + 2 children = 3 visible rows, marker "[-]".
	if len(m.rows) != 3 {
		t.Fatalf("want 3 visible rows expanded, got %d", len(m.rows))
	}
	if !strings.Contains(m.Frame()[2], "[-]") {
		t.Fatalf("expanded group should show [-]: %q", m.Frame()[2])
	}
	// Enter collapses the group under the cursor -> children hidden, marker "[+]".
	m.Update(KeyEvent{Name: "enter"})
	if len(m.rows) != 1 {
		t.Fatalf("collapse should hide children, got %d rows", len(m.rows))
	}
	if !strings.Contains(m.Frame()[2], "[+]") {
		t.Fatalf("collapsed group should show [+]: %q", m.Frame()[2])
	}
	if m.Dirty() {
		t.Fatal("expand/collapse is view-only; must not request a re-query")
	}
	// Right re-expands.
	m.Update(KeyEvent{Name: "right"})
	if len(m.rows) != 3 {
		t.Fatalf("expand should restore children, got %d rows", len(m.rows))
	}
	// Leaves render the blank (non-group) marker slot, not a caret.
	if strings.Contains(m.Frame()[3], "[+]") || strings.Contains(m.Frame()[3], "[-]") {
		t.Fatalf("leaf row must not show a fold marker: %q", m.Frame()[3])
	}
}

func TestModel_GroupCycleIncludesDisplayedDimensions(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 24)
	res, _ := sampleResult(2)
	cols, _ := render.ResolveColumns(metrics.NewDefault(), []string{"target", "pstate", "wchan", "cpu"})
	m.SetResult(res, cols)

	has := func(cycle []groupPreset, gb string) bool {
		for _, p := range cycle {
			if p.groupBy == gb {
				return true
			}
		}
		return false
	}
	cycle := m.groupCycle()
	// Built-in presets stay; displayed groupable columns (pstate/wchan) are added.
	if !has(cycle, "none") || !has(cycle, "comm") {
		t.Fatalf("cycle should keep built-in presets: %+v", cycle)
	}
	if !has(cycle, "wchan") || !has(cycle, "pstate") {
		t.Fatalf("displayed dimension columns should be groupable: %+v", cycle)
	}
	// A numeric metric column is not a dimension and must not be groupable.
	if has(cycle, "cpu") {
		t.Fatalf("a metric column must not be in the group cycle: %+v", cycle)
	}
	// Pressing g eventually reaches the displayed wchan grouping.
	for i := 0; i < len(cycle); i++ {
		m.Update(KeyEvent{Rune: 'g'})
		if m.flags.GroupBy == "wchan" {
			return
		}
	}
	t.Fatalf("g-cycle never reached wchan grouping")
}

func TestModel_ColumnPicker(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(120, 24)
	m.SetColumnChoices([]ColumnChoice{
		{ID: "cpu", Desc: "cpu %"}, {ID: "rss", Desc: "resident memory"}, {ID: "wchan", Desc: "blocking symbol"},
	})
	res, _ := sampleResult(2)
	cols, _ := render.ResolveColumns(metrics.NewDefault(), []string{"target", "cpu"})
	m.SetResult(res, cols)

	// `+` opens the picker; typing filters; Enter toggles the highlighted column on.
	m.Update(KeyEvent{Rune: '+'})
	if !m.colPicker {
		t.Fatal("+ should open the column picker")
	}
	for _, r := range "wch" {
		m.Update(KeyEvent{Rune: r})
	}
	if f := m.filteredChoices(); len(f) != 1 || f[0].ID != "wchan" {
		t.Fatalf("filter should narrow to wchan, got %v", f)
	}
	m.Update(KeyEvent{Name: "enter"}) // toggle wchan on (picker stays open)
	if !m.colPicker {
		t.Fatal("picker should stay open for multi-select")
	}
	if m.flags.Columns != "target,cpu,wchan" {
		t.Fatalf("column not appended: %q", m.flags.Columns)
	}
	// Re-query applies the new set; toggling a shown column removes it.
	cols2, _ := render.ResolveColumns(metrics.NewDefault(), []string{"target", "cpu", "wchan"})
	m.SetResult(res, cols2)
	m.Update(KeyEvent{Name: "enter"}) // wchan still filtered + highlighted → toggle off
	if m.flags.Columns != "target,cpu" {
		t.Fatalf("toggling a shown column should remove it: %q", m.flags.Columns)
	}
	m.Update(KeyEvent{Name: "esc"})
	if m.colPicker {
		t.Fatal("Esc should close the picker")
	}
}

func TestModel_ColumnPickerRenderNavRemove(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(100, 8) // short screen exercises scroll
	choices := make([]ColumnChoice, 0)
	for _, id := range []string{"a", "b", "c", "cpu", "rss", "swap", "wchan", "gpu"} {
		choices = append(choices, ColumnChoice{ID: id, Desc: "desc " + id})
	}
	m.SetColumnChoices(choices)
	res, _ := sampleResult(1)
	cols, _ := render.ResolveColumns(metrics.NewDefault(), []string{"target"})
	m.SetResult(res, cols)

	m.Update(KeyEvent{Rune: '+'})
	joined := strings.Join(m.Frame(), "\n")
	for _, want := range []string{"COLUMNS", "filter>", "[ ]", "cpu"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("picker frame missing %q:\n%s", want, joined)
		}
	}
	// Navigation moves the cursor (and scrolls on a short screen).
	m.Update(KeyEvent{Name: "down"})
	m.Update(KeyEvent{Name: "pgdn"})
	if m.colCursor == 0 {
		t.Fatal("down/pgdn should move the picker cursor")
	}
	// A filter that matches nothing → Enter is a no-op; backspace clears it.
	m.Update(KeyEvent{Rune: 'z'})
	if len(m.filteredChoices()) != 0 {
		t.Fatal("filter 'z' should match nothing")
	}
	m.Update(KeyEvent{Name: "enter"}) // no-op on empty list
	m.Update(KeyEvent{Name: "backspace"})
	if m.colFilter != "" {
		t.Fatalf("backspace should clear the filter, got %q", m.colFilter)
	}
	m.Update(KeyEvent{Name: "esc"})
	if m.colPicker {
		t.Fatal("Esc should close the picker")
	}

	// `-` drops the rightmost column; target is protected.
	c2, _ := render.ResolveColumns(metrics.NewDefault(), []string{"target", "cpu"})
	m.SetResult(res, c2)
	m.Update(KeyEvent{Rune: '-'})
	if m.flags.Columns != "target" {
		t.Fatalf("- should drop cpu, got %q", m.flags.Columns)
	}
	c3, _ := render.ResolveColumns(metrics.NewDefault(), []string{"target"})
	m.SetResult(res, c3)
	m.Update(KeyEvent{Rune: '-'})
	if !strings.Contains(m.status, "required") {
		t.Fatalf("removing target must be refused, status=%q", m.status)
	}
}

func TestModel_FoldAll(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 24)
	res, cols := groupedResult()
	m.SetResult(res, cols)

	// c folds every group at once (view-only, no re-query).
	m.Update(KeyEvent{Rune: 'c'})
	if len(m.rows) != 1 {
		t.Fatalf("fold-all should hide all children, got %d rows", len(m.rows))
	}
	if !strings.Contains(m.Frame()[2], "[+]") {
		t.Fatalf("folded group should show [+]: %q", m.Frame()[2])
	}
	if m.Dirty() {
		t.Fatal("fold-all is view-only; must not request a re-query")
	}
	// The default now folds groups that appear later (regroup/refresh).
	res, cols = groupedResult()
	m.SetResult(res, cols)
	if len(m.rows) != 1 {
		t.Fatalf("groups appearing after fold-all should start folded, got %d rows", len(m.rows))
	}
	// C unfolds every group and clears the fold-all default.
	m.Update(KeyEvent{Rune: 'C'})
	if len(m.rows) != 3 {
		t.Fatalf("unfold-all should reveal children, got %d rows", len(m.rows))
	}
}

func TestModel_CollapseGroupsLaunchFlag(t *testing.T) {
	m := NewModel(queryspec.Flags{CollapseGroups: true})
	m.SetSize(80, 24)
	res, cols := groupedResult()
	m.SetResult(res, cols)
	// --collapse-groups starts every group folded shut.
	if len(m.rows) != 1 {
		t.Fatalf("--collapse-groups should start folded, got %d rows", len(m.rows))
	}
	// A per-group override still wins: Enter expands just this one.
	m.Update(KeyEvent{Name: "enter"})
	if len(m.rows) != 3 {
		t.Fatalf("Enter should expand the folded group despite the default, got %d rows", len(m.rows))
	}
}

func TestModel_FrameAndScroll(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 6) // 2 chrome (status+header) + 4 body rows
	res, cols := sampleResult(10)
	m.SetResult(res, cols)

	frame := m.Frame()
	if len(frame) != 6 {
		t.Fatalf("frame height = %d, want 6", len(frame))
	}
	if !strings.Contains(frame[0], "gen=7") {
		t.Fatalf("status bar missing gen: %q", frame[0])
	}
	if !strings.Contains(frame[1], "TARGET") {
		t.Fatalf("header missing: %q", frame[1])
	}
	// Cursor starts at row 0, marked with '>'.
	if !strings.HasPrefix(frame[2], "> ") {
		t.Fatalf("cursor not on first row: %q", frame[2])
	}
	// Scroll down past the visible window.
	for i := 0; i < 8; i++ {
		m.Update(KeyEvent{Name: "down"})
	}
	if m.cursor != 8 {
		t.Fatalf("cursor = %d, want 8", m.cursor)
	}
	if m.scroll == 0 {
		t.Fatal("expected the view to have scrolled")
	}
}

func TestModel_QuitKeys(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.Update(KeyEvent{Rune: 'q'})
	if !m.Quit() {
		t.Fatal("q should quit")
	}
	m2 := NewModel(queryspec.Flags{})
	m2.Update(KeyEvent{Name: "ctrl-c"})
	if !m2.Quit() {
		t.Fatal("ctrl-c should quit")
	}
}

func TestModel_GroupAndSortAreDirty(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3)
	m.SetResult(res, cols)

	m.Update(KeyEvent{Rune: 'g'})
	if !m.Dirty() {
		t.Fatal("group cycle should mark dirty")
	}
	if m.Flags().GroupBy != "comm" { // first cycle from "processes" -> "by comm"
		t.Fatalf("group-by = %q, want comm", m.Flags().GroupBy)
	}
	m.Update(KeyEvent{Rune: 's'})
	if !m.Dirty() {
		t.Fatal("sort cycle should mark dirty")
	}
	if m.Flags().Sort == "" {
		t.Fatal("sort should be set after cycling")
	}
	// Navigation is NOT dirty.
	m.Update(KeyEvent{Name: "down"})
	if m.Dirty() {
		t.Fatal("navigation must not trigger a re-query")
	}
}

func TestModel_LeafIndependentOfGroup(t *testing.T) {
	m := NewModel(queryspec.Flags{Leaf: "thread"})
	if m.Flags().Leaf != "thread" {
		t.Fatalf("launch --leaf must be honored, got %q", m.Flags().Leaf)
	}
	// Cycling grouping must NOT reset the chosen leaf mode.
	m.Update(KeyEvent{Rune: 'g'})
	if m.Flags().GroupBy != "comm" {
		t.Fatalf("group cycle should set group-by comm, got %q", m.Flags().GroupBy)
	}
	if m.Flags().Leaf != "thread" {
		t.Fatalf("group cycle must preserve leaf, got %q", m.Flags().Leaf)
	}
	// 't' cycles leaf independently: process,thread,none -> from thread to none.
	m.Update(KeyEvent{Rune: 't'})
	if m.Flags().Leaf != "none" || !m.Dirty() {
		t.Fatalf("'t' should cycle leaf thread->none and mark dirty, got %q", m.Flags().Leaf)
	}
}

func TestModel_SetInterval(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetInterval(3 * time.Second)
	if m.IntervalHint() != 3*time.Second {
		t.Fatalf("explicit --interval not honored, got %v", m.IntervalHint())
	}
	// '[' snaps the custom value onto the nearest preset (2s) then steps to 1s.
	m.Update(KeyEvent{Rune: '['})
	if m.IntervalHint() != time.Second {
		t.Fatalf("after '[' want 1s, got %v", m.IntervalHint())
	}
}

func TestModel_IntervalCycle(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	start := m.IntervalHint()
	m.Update(KeyEvent{Rune: ']'})
	if m.IntervalHint() == start {
		t.Fatal("] should change the interval")
	}
}

func TestModel_CLIString(t *testing.T) {
	m := NewModel(queryspec.Flags{GroupBy: "comm", Leaf: "none", Sort: "cpu:desc", Select: "uid == 0"})
	cli := m.CLIString()
	for _, want := range []string{"ps", "--group-by comm", "--leaf none", "--sort cpu:desc", "--select 'uid == 0'"} {
		if !strings.Contains(cli, want) {
			t.Fatalf("CLIString %q missing %q", cli, want)
		}
	}
}

func TestModel_SelectedRow(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3)
	m.SetResult(res, cols)
	_, lbl, ok := m.SelectedRow()
	if !ok || lbl == "" {
		t.Fatalf("selected row wrong: %q ok=%v", lbl, ok)
	}
}
