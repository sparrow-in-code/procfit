package tui

import (
	"strings"

	"github.com/netikras/procfit/internal/meta"
)

// ColumnChoice is one selectable column in the picker: its id and a one-line
// description (from the field catalog), so you browse instead of recall ids.
type ColumnChoice struct {
	ID   string
	Desc string
}

// openColumnPicker opens the interactive column picker (`+`): a type-to-filter
// list of every addable column with its description; Enter toggles the
// highlighted one on/off (stay open to enable several), Esc closes. A re-query
// renders the change — no restart.
func (m *Model) openColumnPicker() {
	if len(m.colChoices) == 0 {
		m.status = "column catalog unavailable"
		return
	}
	m.colPicker = true
	m.colFilter = ""
	m.colCursor = 0
	m.colScroll = 0
}

// colKey handles input while the picker is open.
func (m *Model) colKey(ev KeyEvent) {
	switch {
	case ev.Name == "esc", ev.Name == "ctrl-c":
		m.colPicker = false
		m.status = "columns"
	case ev.Name == "enter":
		m.toggleHighlightedColumn()
	case ev.Name == "up":
		m.moveColCursor(-1)
	case ev.Name == "down":
		m.moveColCursor(1)
	case ev.Name == "pgup":
		m.moveColCursor(-m.colBodyHeight())
	case ev.Name == "pgdn":
		m.moveColCursor(m.colBodyHeight())
	case ev.Name == "backspace":
		if r := []rune(m.colFilter); len(r) > 0 {
			m.colFilter = string(r[:len(r)-1])
			m.colCursor, m.colScroll = 0, 0
		}
	case ev.Rune != 0:
		m.colFilter += string(ev.Rune)
		m.colCursor, m.colScroll = 0, 0
	}
}

// filteredChoices returns the choices whose id or description contains the typed
// filter (case-insensitive); an empty filter shows all.
func (m *Model) filteredChoices() []ColumnChoice {
	f := strings.ToLower(strings.TrimSpace(m.colFilter))
	if f == "" {
		return m.colChoices
	}
	out := make([]ColumnChoice, 0, len(m.colChoices))
	for _, c := range m.colChoices {
		if strings.Contains(strings.ToLower(c.ID), f) || strings.Contains(strings.ToLower(c.Desc), f) {
			out = append(out, c)
		}
	}
	return out
}

func (m *Model) toggleHighlightedColumn() {
	filtered := m.filteredChoices()
	if m.colCursor < 0 || m.colCursor >= len(filtered) {
		return
	}
	id := filtered[m.colCursor].ID
	if m.shownColumns()[id] {
		m.removeColumn(id)
	} else {
		m.appendColumn(id)
	}
}

func (m *Model) shownColumns() map[string]bool {
	shown := make(map[string]bool, len(m.cols))
	for _, c := range m.cols {
		shown[c.ID] = true
	}
	return shown
}

// currentColumnIDs materializes the displayed columns as an explicit id list, so
// add/remove can edit it (columns otherwise come implicitly from the profile).
func (m *Model) currentColumnIDs() []string {
	out := make([]string, 0, len(m.cols))
	for _, c := range m.cols {
		out = append(out, c.ID)
	}
	return out
}

func (m *Model) appendColumn(id string) {
	cols := append(m.currentColumnIDs(), id)
	m.flags.Columns = strings.Join(cols, ",")
	m.status = "added column: " + id
	m.dirty = true
}

// removeColumn drops a column by id, keeping target (the tree label) and never
// leaving the view empty.
func (m *Model) removeColumn(id string) {
	if id == "target" {
		m.status = "target column is required"
		return
	}
	cur := m.currentColumnIDs()
	out := make([]string, 0, len(cur))
	for _, c := range cur {
		if c != id {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		m.status = "cannot remove the last column"
		return
	}
	m.flags.Columns = strings.Join(out, ",")
	m.status = "removed column: " + id
	m.dirty = true
}

// removeLastColumn drops the rightmost column (the `-` shortcut).
func (m *Model) removeLastColumn() {
	cur := m.currentColumnIDs()
	if len(cur) == 0 {
		return
	}
	m.removeColumn(cur[len(cur)-1])
}

func (m *Model) colBodyHeight() int {
	h := m.height - 2 // title + filter line
	if h < 1 {
		return 1
	}
	return h
}

func (m *Model) moveColCursor(delta int) {
	n := len(m.filteredChoices())
	m.colCursor += delta
	if m.colCursor < 0 {
		m.colCursor = 0
	}
	if m.colCursor >= n {
		m.colCursor = maxInt(0, n-1)
	}
	body := m.colBodyHeight()
	if m.colCursor < m.colScroll {
		m.colScroll = m.colCursor
	}
	if m.colCursor >= m.colScroll+body {
		m.colScroll = m.colCursor - body + 1
	}
	if m.colScroll < 0 {
		m.colScroll = 0
	}
}

// colPickerFrame renders the picker overlay: a title, the filter line, then the
// (scrolled) list with a checkbox marking columns already displayed.
func (m *Model) colPickerFrame() []string {
	filtered := m.filteredChoices()
	shown := m.shownColumns()
	title := meta.Name + "  COLUMNS — type to filter · ↑↓ move · Enter toggle · Esc close"
	lines := []string{truncate(title, m.width), truncate("filter> "+m.colFilter+"▏", m.width)}
	body := m.colBodyHeight()
	for i := 0; i < body; i++ {
		idx := m.colScroll + i
		if idx >= len(filtered) {
			lines = append(lines, "")
			continue
		}
		c := filtered[idx]
		mark := "[ ]"
		if shown[c.ID] {
			mark = "[x]"
		}
		cursor := "  "
		if idx == m.colCursor {
			cursor = "> "
		}
		lines = append(lines, truncate(cursor+mark+" "+pad(c.ID, 18, false)+" "+c.Desc, m.width))
	}
	return lines
}
