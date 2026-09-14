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
	cli, err := Run(scr, Deps{Refresh: refresh}, queryspec.Flags{})
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
	requery(m, Deps{Refresh: func(queryspec.Flags) (*query.Result, []render.Column, error) {
		return nil, nil, errBoom{}
	}})
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

func TestModel_PauseToggle(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3)
	m.SetResult(res, cols)
	if m.Paused() {
		t.Fatal("auto-refresh must be on by default")
	}
	// 'p' pauses; pausing alone must not request a re-query.
	m.Update(KeyEvent{Rune: 'p'})
	if !m.Paused() || m.Dirty() {
		t.Fatalf("'p' should pause without marking dirty: paused=%v", m.Paused())
	}
	if !strings.Contains(m.statusBar(), "PAUSED") {
		t.Fatalf("status bar should advertise PAUSED, got %q", m.statusBar())
	}
	// space resumes and re-queries immediately.
	m.Update(KeyEvent{Rune: ' '})
	if m.Paused() || !m.Dirty() {
		t.Fatalf("space should resume and mark dirty: paused=%v", m.Paused())
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
	// The rejection must be VISIBLE: while editing, the status bar has to surface
	// the error, not just the raw edit buffer (otherwise the filter feels inert).
	if !strings.Contains(m.statusBar(), "rss") {
		t.Fatalf("editing status bar must surface the rejection, got %q", m.statusBar())
	}
}

func TestModel_FilterPromptShowsFieldHint(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3) // target, pt, cpu
	m.SetResult(res, cols)
	m.Update(KeyEvent{Rune: '/'})
	// On entry the prompt must list the filterable fields so the user knows what
	// to type; the raw buffer alone is not enough guidance.
	bar := m.statusBar()
	if !strings.Contains(bar, "filter>") || !strings.Contains(bar, "cpu") {
		t.Fatalf("filter prompt should show the field hint (incl. cpu), got %q", bar)
	}
}

func TestModel_FilterBareWordAndHistory(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3) // target, pt, cpu
	m.SetResult(res, cols)

	// A bare word becomes a target substring match (optional quotes).
	m.Update(KeyEvent{Rune: '/'})
	for _, r := range "idea" {
		m.Update(KeyEvent{Rune: r})
	}
	m.Update(KeyEvent{Name: "enter"})
	if got := m.Flags().Having; got != `target contains "idea"` {
		t.Fatalf("bare word should become a target substring, got %q", got)
	}

	// An expression (with operators) passes through unchanged.
	m.Update(KeyEvent{Rune: '/'})
	m.editBuf, m.editPos = "", 0 // clear the seeded buffer
	for _, r := range "cpu>1" {
		m.Update(KeyEvent{Rune: r})
	}
	m.Update(KeyEvent{Name: "enter"})
	if m.Flags().Having != "cpu>1" {
		t.Fatalf("expression filter should pass through, got %q", m.Flags().Having)
	}

	// History recall: Up gives the newest, Up again the older raw input.
	m.Update(KeyEvent{Rune: '/'})
	m.Update(KeyEvent{Name: "up"})
	if m.editBuf != "cpu>1" {
		t.Fatalf("history up should recall newest, got %q", m.editBuf)
	}
	m.Update(KeyEvent{Name: "up"})
	if m.editBuf != "idea" {
		t.Fatalf("history up again should recall older, got %q", m.editBuf)
	}
}

func TestModel_FilterCursorEditing(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3)
	m.SetResult(res, cols)
	m.Update(KeyEvent{Rune: '/'})
	for _, r := range "cpu>5" {
		m.Update(KeyEvent{Rune: r})
	}
	// Left one, insert '1' between '>' and '5'.
	m.Update(KeyEvent{Name: "left"})
	m.Update(KeyEvent{Rune: '1'})
	if m.editBuf != "cpu>15" {
		t.Fatalf("insert at cursor failed: %q", m.editBuf)
	}
	// Home then Delete removes the first char.
	m.Update(KeyEvent{Name: "home"})
	m.Update(KeyEvent{Name: "delete"})
	if m.editBuf != "pu>15" {
		t.Fatalf("delete at cursor failed: %q", m.editBuf)
	}
	// ctrl-right jumps over the whole (space-free) token.
	m.Update(KeyEvent{Name: "home"})
	m.Update(KeyEvent{Name: "ctrl-right"})
	if m.editPos != len([]rune(m.editBuf)) {
		t.Fatalf("ctrl-right should jump a word, pos=%d", m.editPos)
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

func TestModel_ControlNiceFlow(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(3)
	m.SetResult(res, cols)
	var calls []ControlRequest
	m.SetControl(func(req ControlRequest) (string, error) {
		calls = append(calls, req)
		return "ok", nil
	})
	// 'n' -> numeric entry; type 10; Enter -> dry-run preview + confirm gate.
	m.Update(KeyEvent{Rune: 'n'})
	if !m.niceEditing {
		t.Fatal("'n' should start nice entry")
	}
	for _, r := range "10" {
		m.Update(KeyEvent{Rune: r})
	}
	m.Update(KeyEvent{Name: "enter"})
	if !m.confirming {
		t.Fatalf("Enter should open the confirm gate; calls=%+v", calls)
	}
	if len(calls) != 1 || !calls[0].DryRun || calls[0].Kind != CtrlNice || calls[0].Nice != 10 ||
		len(calls[0].PIDs) != 1 || calls[0].PIDs[0] != 1 {
		t.Fatalf("preview call wrong: %+v", calls)
	}
	// 'y' applies (DryRun=false); no apply happens before confirmation.
	m.Update(KeyEvent{Rune: 'y'})
	if m.confirming {
		t.Fatal("'y' should close the confirm gate")
	}
	if len(calls) != 2 || calls[1].DryRun || calls[1].Nice != 10 {
		t.Fatalf("apply call wrong: %+v", calls)
	}
}

func TestModel_ControlGroupCollectsPIDs(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 24)
	res, cols := groupedResult() // group-one with process leaves pid 1 and 2
	m.SetResult(res, cols)
	var got ControlRequest
	m.SetControl(func(req ControlRequest) (string, error) { got = req; return "ok", nil })
	// Cursor is on the group row: control must target ALL member processes.
	m.Update(KeyEvent{Rune: 'x'})
	if !m.confirming {
		t.Fatalf("'x' on a group should open the confirm gate; got %+v", got)
	}
	if len(got.PIDs) != 2 {
		t.Fatalf("group control should collect all member pids, got %v", got.PIDs)
	}
}

func TestModel_ControlForecastView(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 12)
	res, cols := sampleResult(2)
	m.SetResult(res, cols)
	m.SetControl(func(req ControlRequest) (string, error) {
		return "summary line\n  pid 1: 0→10 would apply\n  pid 2: 0→10 would apply", nil
	})
	m.Update(KeyEvent{Rune: 'x'})
	if !m.confirming {
		t.Fatal("'x' should open the confirm gate")
	}
	// The confirm bar shows only the summary line, not the per-pid detail.
	if bar := m.statusBar(); !strings.Contains(bar, "summary line") || strings.Contains(bar, "pid 1:") {
		t.Fatalf("confirm bar should show only the summary: %q", bar)
	}
	// 'v' opens the full forecast overlay.
	m.Update(KeyEvent{Rune: 'v'})
	if !m.previewing {
		t.Fatal("'v' should open the forecast overlay")
	}
	if joined := strings.Join(m.Frame(), "\n"); !strings.Contains(joined, "FORECAST") || !strings.Contains(joined, "pid 2:") {
		t.Fatalf("forecast overlay missing detail:\n%s", joined)
	}
	// Any key returns to the confirm gate.
	m.Update(KeyEvent{Rune: ' '})
	if m.previewing || !m.confirming {
		t.Fatal("a key should return from the overlay to the confirm gate")
	}
}

func TestModel_ControlStopCancel(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(2)
	m.SetResult(res, cols)
	applied := false
	m.SetControl(func(req ControlRequest) (string, error) {
		if !req.DryRun {
			applied = true
		}
		return "ok", nil
	})
	m.Update(KeyEvent{Rune: 'x'})
	if !m.confirming {
		t.Fatal("'x' should open the confirm gate")
	}
	m.Update(KeyEvent{Rune: 'n'}) // decline
	if m.confirming || applied {
		t.Fatalf("declining must not apply: confirming=%v applied=%v", m.confirming, applied)
	}
}

// TestModel_ControlPinnedAcrossReorder proves the confirm gate acts on the pids
// captured when the action began, even if a background refresh reorders the list
// under the cursor (e.g. sorted by cpu%): confirming can never retarget a
// different process than the one previewed.
func TestModel_ControlPinnedAcrossReorder(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 24)
	res, cols := sampleResult(2) // flat rows: index 0 = pid 1, index 1 = pid 2
	m.SetResult(res, cols)
	var got ControlRequest
	m.SetControl(func(req ControlRequest) (string, error) { got = req; return "ok", nil })

	// Aim at pid 1 (cursor index 0) and open the confirm gate.
	m.Update(KeyEvent{Rune: 'x'})
	if !m.confirming || len(got.PIDs) != 1 || got.PIDs[0] != 1 {
		t.Fatalf("stop on pid 1 should preview pid 1, got %+v", got)
	}

	// A background tick refreshes and reorders: pid 2 sorts to index 0 (now under
	// the cursor). The pending action must NOT follow the cursor.
	res2, cols2 := sampleResult(2)
	res2.Rows[0], res2.Rows[1] = res2.Rows[1], res2.Rows[0]
	m.SetResult(res2, cols2)

	m.Update(KeyEvent{Rune: 'y'}) // confirm
	if got.DryRun {
		t.Fatal("confirm must apply (not dry-run)")
	}
	if len(got.PIDs) != 1 || got.PIDs[0] != 1 {
		t.Fatalf("confirm must act on the captured pid 1, not the reordered cursor row, got %+v", got.PIDs)
	}
}

// TestModel_ConfirmSuspendsRefresh checks that a modal control gate freezes
// ticker-driven auto-refresh, so the list can't churn under a pending decision.
func TestModel_ConfirmSuspendsRefresh(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(2)
	m.SetResult(res, cols)
	m.SetControl(func(ControlRequest) (string, error) { return "ok", nil })

	if m.RefreshSuspended() {
		t.Fatal("auto-refresh should run normally before any gate")
	}
	m.Update(KeyEvent{Rune: 'x'}) // open confirm gate
	if !m.RefreshSuspended() {
		t.Fatal("an open confirm gate must suspend auto-refresh")
	}
	m.Update(KeyEvent{Rune: 'n'}) // cancel
	if m.RefreshSuspended() {
		t.Fatal("cancelling the gate must resume auto-refresh")
	}
}

func TestModel_ManagedPanel(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 12)
	res, cols := sampleResult(2) // process rows pid 1, 2
	m.SetResult(res, cols)
	var dropped []int
	m.SetManaged(true, func(pids []int) (string, error) { dropped = pids; return "dropped", nil })

	// Tab -> managed panel: renders the same tree, legend switches to MANAGED.
	m.Update(KeyEvent{Name: "tab"})
	if m.Panel() != PanelManaged {
		t.Fatal("Tab should open the managed panel")
	}
	if !strings.Contains(m.Frame()[0], "MANAGED") {
		t.Fatalf("managed legend expected: %q", m.Frame()[0])
	}
	// 'd' drops the selected row's process(es).
	m.Update(KeyEvent{Rune: 'd'})
	if len(dropped) != 1 || dropped[0] != 1 {
		t.Fatalf("drop should pass the selected pid, got %v", dropped)
	}
	// Tab back to the browser.
	m.Update(KeyEvent{Name: "tab"})
	if m.Panel() != PanelBrowser {
		t.Fatal("Tab should return to the browser")
	}
}

func TestModel_ManagedControlConfirmVisible(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 12)
	res, cols := sampleResult(2)
	m.SetResult(res, cols)
	var got ControlRequest
	m.SetControl(func(req ControlRequest) (string, error) { got = req; return "ok", nil })
	m.SetManaged(true, func([]int) (string, error) { return "", nil })

	m.Update(KeyEvent{Name: "tab"}) // managed panel
	m.Update(KeyEvent{Rune: 'X'})   // continue the selected process
	if !m.confirming {
		t.Fatalf("control in the managed panel should open the confirm gate; got %+v", got)
	}
	if got.Kind != CtrlContinue || len(got.PIDs) != 1 || got.PIDs[0] != 1 {
		t.Fatalf("managed control should target the selected process: %+v", got)
	}
	// The confirm prompt must be visible in the managed panel, not hidden behind
	// the panel legend — otherwise it looks frozen waiting for 'y'.
	if !strings.Contains(m.Frame()[0], "CONFIRM") {
		t.Fatalf("managed panel must show the confirm prompt: %q", m.Frame()[0])
	}
}

func TestModel_DetailOverlay(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 12)
	res, cols := sampleResult(3)
	m.SetResult(res, cols)
	m.Update(KeyEvent{Rune: 'i'})
	if !m.detail {
		t.Fatal("'i' should open the detail overlay")
	}
	fr := m.Frame()
	joined := strings.Join(fr, "\n")
	if !strings.Contains(fr[0], "DETAIL") || !strings.Contains(joined, "pid:") || !strings.Contains(joined, "name:") {
		t.Fatalf("detail overlay missing identity:\n%s", joined)
	}
	m.Update(KeyEvent{Name: "esc"})
	if m.detail {
		t.Fatal("Esc should close the detail overlay")
	}
}

func TestModel_DetailHistory(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	m.SetSize(80, 16)
	res, cols := sampleResult(1) // pid 1
	m.SetResult(res, cols)
	m.SetHistory(func(pid int) []HistoryEntry {
		if pid != 1 {
			return nil
		}
		return []HistoryEntry{{Time: "10:00:00", Action: "applied", Field: "nice", From: "0", To: "5"}}
	})
	m.Update(KeyEvent{Rune: 'i'})
	joined := strings.Join(m.Frame(), "\n")
	if !strings.Contains(joined, "history") || !strings.Contains(joined, "applied") || !strings.Contains(joined, "0→5") {
		t.Fatalf("detail overlay should include control history:\n%s", joined)
	}
}

func TestModel_ControlUnavailable(t *testing.T) {
	m := NewModel(queryspec.Flags{})
	res, cols := sampleResult(2)
	m.SetResult(res, cols)
	m.Update(KeyEvent{Rune: 'n'}) // no controller installed
	if m.niceEditing || m.confirming {
		t.Fatal("control keys must be inert without a controller")
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }
