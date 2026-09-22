package tui

import "strings"

// startAddColumn opens the add-column prompt: type an id (prefix-matched against
// the catalog) and Enter appends it to the right of the current columns, then a
// re-query renders it — no restart needed.
func (m *Model) startAddColumn() {
	if len(m.colChoices) == 0 {
		m.status = "column catalog unavailable"
		return
	}
	m.colEditing = true
	m.colBuf = ""
	m.status = ""
}

// colKey handles input while the add-column prompt is open.
func (m *Model) colKey(ev KeyEvent) {
	switch {
	case ev.Name == "enter":
		id := m.matchColumn(m.colBuf)
		if id == "" {
			m.status = "no addable column matches " + quoteBuf(m.colBuf)
			return
		}
		m.colEditing = false
		m.appendColumn(id)
	case ev.Name == "esc", ev.Name == "ctrl-c":
		m.colEditing = false
		m.status = "add column cancelled"
	case ev.Name == "backspace":
		if r := []rune(m.colBuf); len(r) > 0 {
			m.colBuf = string(r[:len(r)-1])
		}
	case ev.Rune != 0:
		m.colBuf += string(ev.Rune)
	}
}

// matchColumn returns the first not-yet-shown column id whose id has the typed
// prefix (case-insensitive), or "" if none.
func (m *Model) matchColumn(buf string) string {
	shown := m.shownColumns()
	buf = strings.ToLower(strings.TrimSpace(buf))
	for _, id := range m.colChoices {
		if !shown[id] && strings.HasPrefix(id, buf) {
			return id
		}
	}
	return ""
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

// removeLastColumn drops the rightmost column (never the last remaining one).
func (m *Model) removeLastColumn() {
	cols := m.currentColumnIDs()
	if len(cols) <= 1 {
		m.status = "cannot remove the last column"
		return
	}
	dropped := cols[len(cols)-1]
	m.flags.Columns = strings.Join(cols[:len(cols)-1], ",")
	m.status = "removed column: " + dropped
	m.dirty = true
}

func quoteBuf(s string) string {
	if s == "" {
		return "(empty)"
	}
	return "'" + s + "'"
}
