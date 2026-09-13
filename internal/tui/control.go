package tui

import (
	"strconv"
	"strings"
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

// ControlRequest describes one control action on one process. DryRun asks the
// backend to preview (never mutate).
type ControlRequest struct {
	PID    int
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
	if !ok || fr.row.Process == nil {
		m.status = "select a process row (leaf) to control — not a group"
		return
	}
	req := ControlRequest{PID: fr.row.Process.PID, Label: fr.row.Label, Kind: kind}
	if kind == CtrlNice {
		m.pending = req
		m.niceEditing = true
		m.niceBuf = ""
		m.status = "nice value -20..19 (Enter=preview, Esc=cancel)"
		return
	}
	m.beginConfirm(req)
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
	case ev.Name == "esc", ev.Name == "ctrl-c", ev.Rune == 'n', ev.Rune == 'N':
		m.confirming = false
		m.status = "control cancelled"
	}
}
