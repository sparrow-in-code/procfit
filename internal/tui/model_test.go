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
