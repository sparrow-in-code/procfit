package tui

import (
	"fmt"
	"strings"

	"github.com/netikras/procfit/internal/expr"
)

// nameKeys / runeKeys map key events to handlers. Using dispatch tables (rather
// than one large switch) keeps Update small and makes adding a key a one-line
// entry (Open/Closed).
var nameKeys = map[string]func(*Model){
	"ctrl-c": (*Model).quitAction,
	"up":     func(m *Model) { m.moveCursor(-1) },
	"down":   func(m *Model) { m.moveCursor(1) },
	"pgup":   func(m *Model) { m.moveCursor(-m.bodyHeight()) },
	"pgdn":   func(m *Model) { m.moveCursor(m.bodyHeight()) },
}

var runeKeys = map[rune]func(*Model){
	'q': (*Model).quitAction,
	'k': func(m *Model) { m.moveCursor(-1) },
	'j': func(m *Model) { m.moveCursor(1) },
	'g': (*Model).cycleGroup,
	's': (*Model).cycleSort,
	'S': (*Model).toggleSortDir,
	'r': (*Model).refreshAction,
	'u': (*Model).toggleUnits,
	'/': (*Model).startFilter,
	'f': (*Model).startFilter,
	'[': func(m *Model) { m.adjustInterval(-1) },
	']': func(m *Model) { m.adjustInterval(1) },
}

// Update applies a key event, mutating view state. It sets dirty when the query
// must be re-run (grouping/sort/interval/units changes). Navigation is local and
// does not re-query.
func (m *Model) Update(ev KeyEvent) {
	if m.editing {
		m.editKey(ev)
		return
	}
	if h, ok := nameKeys[ev.Name]; ok && ev.Name != "" {
		h(m)
		return
	}
	if h, ok := runeKeys[ev.Rune]; ok {
		h(m)
	}
}

// startFilter enters the interactive filter editor, seeded with the current
// filter. Only displayed, filterable columns may be referenced (validated on
// apply), so you can only filter by properties actually on screen.
func (m *Model) startFilter() {
	m.editing = true
	m.editBuf = m.flags.Having
	m.status = "filter (fields: " + strings.Join(m.filterableList(), " ") + ") — Enter=apply Esc=cancel"
}

func (m *Model) editKey(ev KeyEvent) {
	switch {
	case ev.Name == "enter":
		m.applyFilter()
	case ev.Name == "esc", ev.Name == "ctrl-c":
		m.editing = false
		m.status = "filter cancelled"
	case ev.Name == "backspace":
		if r := []rune(m.editBuf); len(r) > 0 {
			m.editBuf = string(r[:len(r)-1])
		}
	case ev.Rune != 0:
		m.editBuf += string(ev.Rune)
	}
}

func (m *Model) applyFilter() {
	s := strings.TrimSpace(m.editBuf)
	if s == "" {
		m.editing = false
		m.flags.Having = ""
		m.status = "filter cleared"
		m.dirty = true
		return
	}
	prog, err := expr.Compile(s)
	if err != nil {
		m.status = "filter error: " + err.Error()
		return // stay in edit mode so the user can fix it
	}
	if err := prog.Validate(m.filterableFields()); err != nil {
		m.status = "filter: " + err.Error() + " (only displayed columns)"
		return
	}
	m.editing = false
	m.flags.Having = s
	m.status = "filter: " + s
	m.dirty = true
}

func (m *Model) filterableList() []string {
	var out []string
	for _, c := range m.cols {
		if c.Filterable {
			out = append(out, c.ID)
		}
	}
	return out
}

func (m *Model) quitAction() { m.quit = true }

func (m *Model) refreshAction() {
	m.dirty = true
	m.status = "refreshing"
}

func (m *Model) toggleUnits() {
	m.flags.Human = !m.flags.Human
	m.status = "units: " + unitsLabel(m.flags.Human)
	m.dirty = true
}

func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = maxInt(0, len(m.rows)-1)
	}
	m.clampScroll()
}

func (m *Model) clampScroll() {
	body := m.bodyHeight()
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+body {
		m.scroll = m.cursor - body + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *Model) cycleGroup() {
	m.groupIx = (m.groupIx + 1) % len(groupPresets)
	p := groupPresets[m.groupIx]
	m.flags.GroupBy = p.groupBy
	m.flags.Leaf = p.leaf
	m.cursor, m.scroll = 0, 0
	m.status = "group: " + p.label
	m.dirty = true
}

// cycleSort advances the sort key to the next SORTABLE displayed column, so the
// user can only sort by properties actually shown (and that the engine supports).
func (m *Model) cycleSort() {
	n := len(m.cols)
	for i := 1; i <= n; i++ {
		idx := (m.sortIx + i) % n
		if m.cols[idx].Sortable {
			m.sortIx = idx
			m.applySort()
			return
		}
	}
	m.status = "no sortable columns"
}

func (m *Model) toggleSortDir() {
	m.sortDsc = !m.sortDsc
	m.applySort()
}

func (m *Model) applySort() {
	if len(m.cols) == 0 {
		return
	}
	field := m.cols[m.sortIx].ID
	dir := "asc"
	if m.sortDsc {
		dir = "desc"
	}
	m.flags.Sort = field + ":" + dir
	m.status = "sort: " + m.flags.Sort
	m.dirty = true
}

func (m *Model) adjustInterval(step int) {
	// Interval is advisory here; the driver reads Flags-independent interval via
	// IntervalHint. Kept simple: cycle common intervals.
	m.intervalStep += step
	m.status = fmt.Sprintf("interval: %s", m.IntervalHint())
}

func unitsLabel(human bool) string {
	if human {
		return "human (K/M/G)"
	}
	return "raw bytes"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
