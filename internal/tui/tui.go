package tui

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/render"
)

// RefreshFunc runs a query for the given flags and returns the result plus the
// resolved columns. The TUI depends only on this callback, not on the engine or
// daemon — so the same UI works embedded or (later) over the daemon.
type RefreshFunc func(queryspec.Flags) (*query.Result, []render.Column, error)

// RunTerminal creates a real terminal screen, runs the explorer, and returns the
// equivalent CLI command for the final view. It keeps tcell setup inside this
// package so callers depend only on RefreshFunc.
// Deps are the backend callbacks the explorer needs. Only Refresh is required;
// Control/Managed enable the control side plane when present.
type Deps struct {
	Refresh     RefreshFunc // browser data source
	ManagedTree RefreshFunc // managed panel data source (same shape; managed subset + orphans)
	Control     ControlFunc
	Drop        DropFunc // unmanage the target(s) owning the selected pids
	History     HistoryFunc
	Interval    time.Duration // starting refresh interval (0 = default 1s)
}

func RunTerminal(deps Deps, flags queryspec.Flags) (string, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return "", err
	}
	if err := screen.Init(); err != nil {
		return "", err
	}
	defer screen.Fini()
	return Run(screen, deps, flags)
}

// Run drives the interactive explorer on the given screen until the user quits,
// returning the equivalent CLI command for the final view (Phase 3 exit
// criterion). Sampling (via refresh on a ticker) and input are handled in one
// loop so slow refreshes never wedge input handling (RFC §19.4).
func Run(screen tcell.Screen, deps Deps, flags queryspec.Flags) (string, error) {
	m := NewModel(flags)
	m.SetControl(deps.Control)
	m.SetManaged(deps.ManagedTree != nil, deps.Drop)
	m.SetHistory(deps.History)
	m.SetInterval(deps.Interval)
	w, h := screen.Size()
	m.SetSize(w, h)
	requery(m, deps)

	events := make(chan tcell.Event, 16)
	go pollEvents(screen, events)

	interval := m.IntervalHint()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	draw(screen, m)

	for !m.Quit() {
		select {
		case ev := <-events:
			handleEvent(screen, m, ev, deps)
		case <-ticker.C:
			if !m.Paused() {
				requery(m, deps)
			}
		}
		if ni := m.IntervalHint(); ni != interval {
			interval = ni
			ticker.Reset(interval)
		}
		draw(screen, m)
	}
	return m.CLIString(), nil
}

func pollEvents(screen tcell.Screen, out chan<- tcell.Event) {
	for {
		ev := screen.PollEvent()
		if ev == nil {
			return
		}
		out <- ev
	}
}

func handleEvent(screen tcell.Screen, m *Model, ev tcell.Event, deps Deps) {
	switch e := ev.(type) {
	case *tcell.EventResize:
		w, h := e.Size()
		m.SetSize(w, h)
		screen.Sync()
	case *tcell.EventKey:
		m.Update(mapKey(e))
		if m.Dirty() {
			requery(m, deps)
		}
	}
}

// requery re-queries the active panel's data source (browser or managed).
func requery(m *Model, deps Deps) {
	fn := deps.Refresh
	if m.Panel() == PanelManaged && deps.ManagedTree != nil {
		fn = deps.ManagedTree
	}
	res, cols, err := fn(m.Flags())
	if err != nil {
		m.status = "error: " + err.Error()
		return
	}
	m.SetResult(res, cols)
}

// keyNames maps the plain (unmodified) tcell keys to terminal-agnostic names.
// Kept as data so mapKey stays small; modified/rune keys are handled separately.
var keyNames = map[tcell.Key]string{
	tcell.KeyCtrlC:      "ctrl-c",
	tcell.KeyUp:         "up",
	tcell.KeyDown:       "down",
	tcell.KeyPgUp:       "pgup",
	tcell.KeyPgDn:       "pgdn",
	tcell.KeyHome:       "home",
	tcell.KeyEnd:        "end",
	tcell.KeyDelete:     "delete",
	tcell.KeyEnter:      "enter",
	tcell.KeyTab:        "tab",
	tcell.KeyEscape:     "esc",
	tcell.KeyBackspace:  "backspace",
	tcell.KeyBackspace2: "backspace",
}

func mapKey(ev *tcell.EventKey) KeyEvent {
	ctrl := ev.Modifiers()&tcell.ModCtrl != 0
	switch ev.Key() {
	case tcell.KeyLeft:
		return KeyEvent{Name: ctrlName("left", ctrl)}
	case tcell.KeyRight:
		return KeyEvent{Name: ctrlName("right", ctrl)}
	case tcell.KeyRune:
		return KeyEvent{Rune: ev.Rune()}
	}
	if name, ok := keyNames[ev.Key()]; ok {
		return KeyEvent{Name: name}
	}
	return KeyEvent{}
}

func ctrlName(base string, ctrl bool) string {
	if ctrl {
		return "ctrl-" + base
	}
	return base
}

func draw(screen tcell.Screen, m *Model) {
	screen.Clear()
	style := tcell.StyleDefault
	header := style.Bold(true)
	for y, line := range m.Frame() {
		st := style
		if y <= 1 {
			st = header
		}
		for x, r := range line {
			screen.SetContent(x, y, r, nil, st)
		}
	}
	screen.Show()
}
