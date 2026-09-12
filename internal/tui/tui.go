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
func RunTerminal(refresh RefreshFunc, flags queryspec.Flags) (string, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return "", err
	}
	if err := screen.Init(); err != nil {
		return "", err
	}
	defer screen.Fini()
	return Run(screen, refresh, flags)
}

// Run drives the interactive explorer on the given screen until the user quits,
// returning the equivalent CLI command for the final view (Phase 3 exit
// criterion). Sampling (via refresh on a ticker) and input are handled in one
// loop so slow refreshes never wedge input handling (RFC §19.4).
func Run(screen tcell.Screen, refresh RefreshFunc, flags queryspec.Flags) (string, error) {
	m := NewModel(flags)
	w, h := screen.Size()
	m.SetSize(w, h)
	requery(m, refresh)

	events := make(chan tcell.Event, 16)
	go pollEvents(screen, events)

	interval := m.IntervalHint()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	draw(screen, m)

	for !m.Quit() {
		select {
		case ev := <-events:
			handleEvent(screen, m, ev, refresh)
		case <-ticker.C:
			requery(m, refresh)
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

func handleEvent(screen tcell.Screen, m *Model, ev tcell.Event, refresh RefreshFunc) {
	switch e := ev.(type) {
	case *tcell.EventResize:
		w, h := e.Size()
		m.SetSize(w, h)
		screen.Sync()
	case *tcell.EventKey:
		m.Update(mapKey(e))
		if m.Dirty() {
			requery(m, refresh)
		}
	}
}

func requery(m *Model, refresh RefreshFunc) {
	res, cols, err := refresh(m.Flags())
	if err != nil {
		m.status = "error: " + err.Error()
		return
	}
	m.SetResult(res, cols)
}

func mapKey(ev *tcell.EventKey) KeyEvent {
	switch ev.Key() {
	case tcell.KeyCtrlC:
		return KeyEvent{Name: "ctrl-c"}
	case tcell.KeyUp:
		return KeyEvent{Name: "up"}
	case tcell.KeyDown:
		return KeyEvent{Name: "down"}
	case tcell.KeyPgUp:
		return KeyEvent{Name: "pgup"}
	case tcell.KeyPgDn:
		return KeyEvent{Name: "pgdn"}
	case tcell.KeyEnter:
		return KeyEvent{Name: "enter"}
	case tcell.KeyEscape:
		return KeyEvent{Name: "esc"}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return KeyEvent{Name: "backspace"}
	case tcell.KeyRune:
		return KeyEvent{Rune: ev.Rune()}
	default:
		return KeyEvent{}
	}
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
