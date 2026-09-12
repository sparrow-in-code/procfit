package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/render"
)

// TestRun_SimulationScreen drives the tcell driver headlessly via a simulation
// screen, exercising Run/draw/mapKey/handleEvent/pollEvents/requery.
func TestRun_SimulationScreen(t *testing.T) {
	scr := tcell.NewSimulationScreen("")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	scr.SetSize(80, 24)
	res, cols := sampleResult(5)
	refresh := func(queryspec.Flags) (*query.Result, []render.Column, error) {
		return res, cols, nil
	}
	go func() {
		time.Sleep(30 * time.Millisecond)
		scr.InjectKey(tcell.KeyRune, 'g', tcell.ModNone) // triggers a re-query
		scr.InjectKey(tcell.KeyDown, 0, tcell.ModNone)   // navigation
		time.Sleep(10 * time.Millisecond)
		scr.InjectKey(tcell.KeyRune, 'q', tcell.ModNone) // quit
	}()
	cli, err := Run(scr, refresh, queryspec.Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if cli == "" {
		t.Fatal("Run should return the reproducible CLI on exit")
	}
}

// TestRun_RefreshError verifies a failing refresh sets a status without crashing.
func TestRun_RefreshError(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	requery(m, func(queryspec.Flags) (*query.Result, []render.Column, error) {
		return nil, nil, errBoom{}
	})
	// The model should still be usable (no panic); status reflects the error.
	if m.Quit() {
		t.Fatal("refresh error must not quit")
	}
}

func TestModel_UnitsToggle(t *testing.T) {
	m := NewModel(queryspec.Flags{}) // default: raw
	if m.Flags().Human {
		t.Fatal("human must be off by default in the TUI")
	}
	m.Update(KeyEvent{Rune: 'u'})
	if !m.Flags().Human || !m.Dirty() {
		t.Fatal("'u' should enable human units and mark dirty")
	}
	m.Update(KeyEvent{Rune: 'u'})
	if m.Flags().Human {
		t.Fatal("'u' should toggle back to raw")
	}
}

func TestModel_SortOnlyDisplayedSortable(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3) // columns: target, pt, cpu (pt is not sortable)
	m.SetResult(res, cols)
	// From target (sortable), cycling must skip pt and land on cpu.
	m.Update(KeyEvent{Rune: 's'})
	if !strings.HasPrefix(m.Flags().Sort, "cpu:") {
		t.Fatalf("sort should skip the non-sortable pt column and use cpu, got %q", m.Flags().Sort)
	}
}

func TestModel_FilterOnlyDisplayedColumns(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3) // target, pt, cpu
	m.SetResult(res, cols)

	// Enter filter mode and type a filter over a displayed column.
	m.Update(KeyEvent{Rune: '/'})
	if !m.editing {
		t.Fatal("'/' should enter filter-edit mode")
	}
	for _, r := range "cpu>5" {
		m.Update(KeyEvent{Rune: r})
	}
	m.Update(KeyEvent{Name: "backspace"}) // delete '5'
	m.Update(KeyEvent{Rune: '9'})
	m.Update(KeyEvent{Name: "enter"})
	if m.editing || m.Flags().Having != "cpu>9" || !m.Dirty() {
		t.Fatalf("applying a displayed-column filter failed: editing=%v having=%q", m.editing, m.Flags().Having)
	}

	// A filter referencing a NON-displayed column must be rejected.
	m.Update(KeyEvent{Rune: '/'})
	m.editBuf = "rss > 1" // rss is not a displayed column
	m.Update(KeyEvent{Name: "enter"})
	if !m.editing {
		t.Fatal("filter on a hidden column should be rejected and stay in edit mode")
	}
	if m.Flags().Having != "cpu>9" {
		t.Fatalf("rejected filter must not change the active filter, got %q", m.Flags().Having)
	}
	if !strings.Contains(m.status, "rss") {
		t.Fatalf("status should explain the invalid field: %q", m.status)
	}
}

func TestModel_FilterCancelAndClear(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3)
	m.SetResult(res, cols)
	m.flags.Having = "cpu > 1"
	// Cancel keeps the existing filter.
	m.Update(KeyEvent{Rune: '/'})
	m.Update(KeyEvent{Name: "esc"})
	if m.editing || m.Flags().Having != "cpu > 1" {
		t.Fatal("esc should cancel edit and keep the filter")
	}
	// Empty apply clears the filter.
	m.Update(KeyEvent{Rune: '/'})
	m.editBuf = ""
	m.Update(KeyEvent{Name: "enter"})
	if m.Flags().Having != "" {
		t.Fatalf("empty filter should clear, got %q", m.Flags().Having)
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }
