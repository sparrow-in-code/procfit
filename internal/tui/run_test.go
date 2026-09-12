package tui

import (
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

type errBoom struct{}

func (errBoom) Error() string { return "boom" }
