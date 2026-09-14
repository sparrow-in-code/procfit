package tui

import (
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/query"
)

// ControlKind enumerates the throttling/control actions the explorer can request
// on the selected process.
type ControlKind int

const (
	CtrlNice ControlKind = iota
	CtrlStop
	CtrlContinue
	CtrlFreeze
	CtrlThaw
	CtrlRestore
)

// Verb is a human-readable description of the action, for previews/prompts.
func (k ControlKind) Verb() string {
	switch k {
	case CtrlNice:
		return "renice"
	case CtrlStop:
		return "stop (SIGSTOP)"
	case CtrlContinue:
		return "continue (SIGCONT)"
	case CtrlFreeze:
		return "freeze"
	case CtrlThaw:
		return "thaw"
	case CtrlRestore:
		return "restore"
	}
	return "?"
}

// ControlRequest describes a control action over one or more processes (a single
// leaf, or every process under a selected group). DryRun asks the backend to
// preview (never mutate).
type ControlRequest struct {
	PIDs   []int
	Label  string
	Kind   ControlKind
	Nice   int
	DryRun bool
}

// ControlFunc applies (or, with DryRun, previews) a control action and returns a
// one-line summary. The TUI depends only on this callback, never on the control
// plane — keeping observation and control decoupled and the UI testable.
type ControlFunc func(ControlRequest) (string, error)

// startControl begins a control action on the selected process row. Nice needs a
// value first (a small numeric prompt); the others go straight to the mandatory
// confirmation preview.
func (m *Model) startControl(kind ControlKind) {
	if m.control == nil {
		m.status = "control unavailable"
		return
	}
	fr, ok := m.currentRow()
	if !ok {
		return
	}
	pids := collectPIDs(fr.row)
	if len(pids) == 0 {
		m.status = "no controllable processes under the selection"
		return
	}
	req := ControlRequest{PIDs: pids, Label: fr.row.Label, Kind: kind}
	if kind == CtrlNice {
		m.pending = req
		m.niceEditing = true
		m.niceBuf = ""
		m.status = "nice value -20..19 (Enter=preview, Esc=cancel)"
		return
	}
	m.beginConfirm(req)
}

// collectPIDs gathers the distinct owning process ids under a row: just the
// process for a leaf, or every descendant process for a (sub)group — so control
// on a group row throttles the whole group. Thread rows map to their owner pid.
func collectPIDs(r *query.Row) []int {
	seen := map[int]bool{}
	var out []int
	var walk func(n *query.Row)
	walk = func(n *query.Row) {
		pid := 0
		switch {
		case n.Process != nil:
			pid = n.Process.PID
		case n.Thread != nil:
			pid = n.Thread.ID.Process.PID
		}
		if pid != 0 && !seen[pid] {
			seen[pid] = true
			out = append(out, pid)
		}
		for _, c := range n.Sub {
			walk(c)
		}
	}
	walk(r)
	return out
}

func (m *Model) niceKey(ev KeyEvent) {
	switch {
	case ev.Name == "enter":
		n, err := strconv.Atoi(strings.TrimSpace(m.niceBuf))
		if err != nil {
			m.status = "nice must be an integer -20..19"
			return
		}
		m.niceEditing = false
		m.pending.Nice = n
		m.beginConfirm(m.pending)
	case ev.Name == "esc", ev.Name == "ctrl-c":
		m.niceEditing = false
		m.status = "cancelled"
	case ev.Name == "backspace":
		if r := []rune(m.niceBuf); len(r) > 0 {
			m.niceBuf = string(r[:len(r)-1])
		}
	case ev.Rune == '-' || (ev.Rune >= '0' && ev.Rune <= '9'):
		m.niceBuf += string(ev.Rune)
	}
}

// beginConfirm runs the action in dry-run to build a preview and enters the
// confirmation gate; nothing is applied until the user confirms (RFC §19.3).
func (m *Model) beginConfirm(req ControlRequest) {
	req.DryRun = true
	preview, err := m.control(req)
	if err != nil {
		m.status = "control: " + err.Error()
		return
	}
	m.pending = req
	m.preview = preview
	m.confirming = true
	m.previewing = false
}

func (m *Model) confirmKey(ev KeyEvent) {
	switch {
	case ev.Rune == 'y' || ev.Rune == 'Y':
		req := m.pending
		req.DryRun = false
		res, err := m.control(req)
		m.confirming = false
		if err != nil {
			m.status = "control: " + err.Error()
			return
		}
		m.status = res
		m.dirty = true // reflect the change on the next refresh
	case ev.Rune == 'v', ev.Rune == 'V':
		m.previewing = true // show the full per-pid forecast
	case ev.Name == "esc", ev.Name == "ctrl-c", ev.Rune == 'n', ev.Rune == 'N':
		m.confirming = false
		m.status = "control cancelled"
	}
}
