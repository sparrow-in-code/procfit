package tui

import "fmt"

// Update applies a key event, mutating view state. It sets dirty when the query
// must be re-run (grouping/sort/interval changes). Navigation is local and does
// not re-query.
func (m *Model) Update(ev KeyEvent) {
	switch {
	case ev.Name == "ctrl-c", ev.Rune == 'q':
		m.quit = true
	case ev.Name == "up", ev.Rune == 'k':
		m.moveCursor(-1)
	case ev.Name == "down", ev.Rune == 'j':
		m.moveCursor(1)
	case ev.Name == "pgup":
		m.moveCursor(-m.bodyHeight())
	case ev.Name == "pgdn":
		m.moveCursor(m.bodyHeight())
	case ev.Rune == 'g':
		m.cycleGroup()
	case ev.Rune == 's':
		m.cycleSort()
	case ev.Rune == 'S':
		m.toggleSortDir()
	case ev.Rune == 'r':
		m.dirty = true
		m.status = "refreshing"
	case ev.Rune == '[':
		m.adjustInterval(-1)
	case ev.Rune == ']':
		m.adjustInterval(1)
	}
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

// sortableFields cycles the sort key across the current columns.
func (m *Model) cycleSort() {
	if len(m.cols) == 0 {
		return
	}
	m.sortIx = (m.sortIx + 1) % len(m.cols)
	m.applySort()
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
