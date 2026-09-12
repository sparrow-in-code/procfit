// Package tui is the interactive explorer. All view state and key handling live
// in Model, which is headless and unit-testable; the tcell driver (tui.go) only
// pumps events and paints Model.Frame(). This keeps the terminal-framework choice
// isolated and reversible (DEVELOPMENT.md §2.4).
package tui

import (
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/render"
)

// KeyEvent is a terminal-agnostic key press, so the model needs no tcell import.
type KeyEvent struct {
	Rune rune
	Name string // "up","down","pgup","pgdn","enter","esc","ctrl-c",""
}

type groupPreset struct {
	label   string
	groupBy string
	leaf    string
}

var groupPresets = []groupPreset{
	{"processes", "none", "process"},
	{"by comm", "comm", "none"},
	{"by user", "user", "none"},
	{"by app", "app", "none"},
	{"by namespace-set", "namespace-set,comm", "none"},
}

// Model holds all interactive state.
type Model struct {
	flags        queryspec.Flags
	result       *query.Result
	cols         []render.Column
	rows         []flatRow
	width        int
	height       int
	scroll       int
	cursor       int
	groupIx      int
	sortIx       int
	sortDsc      bool
	intervalStep int
	status       string
	quit         bool
	dirty        bool // a re-query is needed
}

type flatRow struct {
	row   *query.Row
	depth int
}

// NewModel builds a model with initial flags.
func NewModel(flags queryspec.Flags) *Model {
	m := &Model{flags: flags, height: 24, width: 100}
	if flags.Sort != "" {
		m.status = "sorted by " + flags.Sort
	}
	return m
}

// Flags returns the current query flags (for the sampler).
func (m *Model) Flags() queryspec.Flags { return m.flags }

// Quit reports whether the user asked to exit.
func (m *Model) Quit() bool { return m.quit }

// Dirty reports (and clears) whether a re-query is needed.
func (m *Model) Dirty() bool {
	d := m.dirty
	m.dirty = false
	return d
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

// SetResult stores the latest query result and its columns.
func (m *Model) SetResult(res *query.Result, cols []render.Column) {
	m.result = res
	m.cols = cols
	m.rows = flatten(res.Rows, 0)
	if m.cursor >= len(m.rows) {
		m.cursor = maxInt(0, len(m.rows)-1)
	}
	m.clampScroll()
}

func flatten(rows []*query.Row, depth int) []flatRow {
	var out []flatRow
	for _, r := range rows {
		out = append(out, flatRow{row: r, depth: depth})
		out = append(out, flatten(r.Sub, depth+1)...)
	}
	return out
}
