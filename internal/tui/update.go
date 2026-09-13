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
	"enter":  (*Model).toggleCollapse,
	"left":   (*Model).collapseOrParent,
	"right":  (*Model).expandOrChild,
}

var runeKeys = map[rune]func(*Model){
	'q': (*Model).quitAction,
	'k': func(m *Model) { m.moveCursor(-1) },
	'j': func(m *Model) { m.moveCursor(1) },
	'g': (*Model).cycleGroup,
	't': (*Model).cycleLeaf,
	's': (*Model).cycleSort,
	'S': (*Model).toggleSortDir,
	'r': (*Model).refreshAction,
	'p': (*Model).togglePause,
	' ': (*Model).togglePause,
	'u': (*Model).toggleUnits,
	'n': func(m *Model) { m.startControl(CtrlNice) },
	'x': func(m *Model) { m.startControl(CtrlStop) },
	'c': func(m *Model) { m.startControl(CtrlContinue) },
	'z': func(m *Model) { m.startControl(CtrlFreeze) },
	'Z': func(m *Model) { m.startControl(CtrlThaw) },
	'R': func(m *Model) { m.startControl(CtrlRestore) },
	'/': (*Model).startFilter,
	'f': (*Model).startFilter,
	'[': func(m *Model) { m.adjustInterval(-1) },
	']': func(m *Model) { m.adjustInterval(1) },
}

// Update applies a key event, mutating view state. It sets dirty when the query
// must be re-run (grouping/sort/interval/units changes). Navigation is local and
// does not re-query.
func (m *Model) Update(ev KeyEvent) {
	if m.confirming {
		m.confirmKey(ev)
		return
	}
	if m.niceEditing {
		m.niceKey(ev)
		return
	}
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
	m.editPos = len([]rune(m.editBuf))
	m.histIdx = len(m.history)
	m.status = "filter (fields: " + strings.Join(m.filterableList(), " ") +
		") — bare word = target substring; && || to chain; Enter=apply Esc=cancel"
}

func (m *Model) editKey(ev KeyEvent) {
	if m.editNav(ev) {
		return
	}
	switch {
	case ev.Name == "enter":
		m.applyFilter()
	case ev.Name == "esc", ev.Name == "ctrl-c":
		m.editing = false
		m.status = "filter cancelled"
	case ev.Name == "backspace":
		m.editBackspace()
	case ev.Name == "delete":
		m.editDelete()
	case ev.Rune != 0:
		m.editInsert(ev.Rune)
	}
}

// editNav handles cursor movement and history recall while editing; it returns
// true when it consumed the event.
func (m *Model) editNav(ev KeyEvent) bool {
	r := []rune(m.editBuf)
	switch ev.Name {
	case "left":
		m.editPos = maxInt(0, m.editPos-1)
	case "right":
		m.editPos = minInt(len(r), m.editPos+1)
	case "home":
		m.editPos = 0
	case "end":
		m.editPos = len(r)
	case "ctrl-left":
		m.editPos = wordLeft(r, m.editPos)
	case "ctrl-right":
		m.editPos = wordRight(r, m.editPos)
	case "up":
		m.historyMove(-1)
	case "down":
		m.historyMove(1)
	default:
		return false
	}
	return true
}

func (m *Model) editInsert(rn rune) {
	r := []rune(m.editBuf)
	if m.editPos > len(r) {
		m.editPos = len(r)
	}
	r = append(r[:m.editPos], append([]rune{rn}, r[m.editPos:]...)...)
	m.editBuf = string(r)
	m.editPos++
}

func (m *Model) editBackspace() {
	r := []rune(m.editBuf)
	if m.editPos == 0 || len(r) == 0 {
		return
	}
	r = append(r[:m.editPos-1], r[m.editPos:]...)
	m.editBuf = string(r)
	m.editPos--
}

func (m *Model) editDelete() {
	r := []rune(m.editBuf)
	if m.editPos >= len(r) {
		return
	}
	m.editBuf = string(append(r[:m.editPos], r[m.editPos+1:]...))
}

// historyMove browses applied filters: dir -1 = older, +1 = newer. Moving past
// the newest returns to an empty "live" line.
func (m *Model) historyMove(dir int) {
	m.histIdx += dir
	if m.histIdx < 0 {
		m.histIdx = 0
	}
	if m.histIdx >= len(m.history) {
		m.histIdx = len(m.history)
		m.editBuf = ""
	} else {
		m.editBuf = m.history[m.histIdx]
	}
	m.editPos = len([]rune(m.editBuf))
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
	full := bareSearch(s) // a plain word becomes a target substring match
	prog, err := expr.Compile(full)
	if err != nil {
		m.status = "filter error: " + err.Error()
		return // stay in edit mode so the user can fix it
	}
	if err := prog.Validate(m.filterableFields()); err != nil {
		m.status = "filter: " + err.Error() + " (only displayed columns)"
		return
	}
	m.editing = false
	m.flags.Having = full
	m.pushHistory(s)
	m.status = "filter: " + full
	m.dirty = true
}

// pushHistory appends the raw input to the history ring (deduping the most
// recent) and resets the browse position to live.
func (m *Model) pushHistory(s string) {
	if n := len(m.history); n == 0 || m.history[n-1] != s {
		m.history = append(m.history, s)
	}
	m.histIdx = len(m.history)
}

// bareSearch turns a plain search term (no operators/keywords) into a target
// substring match, so `idea` means `target contains "idea"` — less boilerplate.
// Anything that already looks like an expression is passed through unchanged.
func bareSearch(s string) string {
	if strings.ContainsAny(s, "=<>~!()[]\"'") || strings.Contains(s, "&&") || strings.Contains(s, "||") {
		return s
	}
	for _, w := range strings.Fields(s) {
		switch w {
		case "contains", "in", "not", "and", "or":
			return s
		}
	}
	return `target contains "` + s + `"`
}

func wordLeft(r []rune, pos int) int {
	i := pos
	for i > 0 && isEditSpace(r[i-1]) {
		i--
	}
	for i > 0 && !isEditSpace(r[i-1]) {
		i--
	}
	return i
}

func wordRight(r []rune, pos int) int {
	i, n := pos, len(r)
	for i < n && isEditSpace(r[i]) {
		i++
	}
	for i < n && !isEditSpace(r[i]) {
		i++
	}
	return i
}

func isEditSpace(r rune) bool { return r == ' ' || r == '\t' }

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

// togglePause suspends/resumes ticker-driven auto-refresh so the user can settle
// the cursor on a target without it moving. Resuming re-queries immediately.
func (m *Model) togglePause() {
	m.paused = !m.paused
	if m.paused {
		m.status = "paused — auto-refresh off (p/space resumes, r refreshes once)"
		return
	}
	m.status = "resumed"
	m.dirty = true
}

func (m *Model) toggleUnits() {
	m.flags.Human = !m.flags.Human
	m.status = "units: " + unitsLabel(m.flags.Human)
	m.dirty = true
}

// toggleCollapse folds/unfolds the (sub)group under the cursor. Leaves are
// ignored. This is a view-only change (no re-query), so it works while paused.
func (m *Model) toggleCollapse() {
	fr, ok := m.currentRow()
	if !ok {
		return
	}
	if !fr.hasKids {
		if m.hasGroups {
			m.status = "not a group — nothing to fold"
		} else {
			m.status = "flat list — press g to group, then Enter folds"
		}
		return
	}
	m.setCollapsed(fr.path, !m.collapsed[fr.path])
}

// collapseOrParent folds an open group; on a leaf or already-folded group it
// jumps to the parent group (like a file tree's Left key).
func (m *Model) collapseOrParent() {
	fr, ok := m.currentRow()
	if !ok {
		return
	}
	if fr.hasKids && !m.collapsed[fr.path] {
		m.setCollapsed(fr.path, true)
		return
	}
	m.moveToParent()
}

// expandOrChild unfolds a folded group; on an already-open group it steps into
// the first child (like a file tree's Right key).
func (m *Model) expandOrChild() {
	fr, ok := m.currentRow()
	if !ok || !fr.hasKids {
		return
	}
	if m.collapsed[fr.path] {
		m.setCollapsed(fr.path, false)
		return
	}
	m.moveCursor(1)
}

func (m *Model) setCollapsed(path string, v bool) {
	m.collapsed[path] = v
	m.status = "expanded"
	if v {
		m.status = "collapsed"
	}
	m.rebuildRows()
}

func (m *Model) currentRow() (flatRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return flatRow{}, false
	}
	return m.rows[m.cursor], true
}

func (m *Model) moveToParent() {
	cur, ok := m.currentRow()
	if !ok {
		return
	}
	for i := m.cursor - 1; i >= 0; i-- {
		if m.rows[i].depth < cur.depth {
			m.cursor = i
			m.clampScroll()
			return
		}
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
	m.cursor, m.scroll = 0, 0
	m.status = "group: " + p.label
	m.dirty = true
}

// cycleLeaf changes the terminal-row mode (process/thread/none) independently of
// grouping, so switching groups never clobbers a chosen --leaf.
func (m *Model) cycleLeaf() {
	m.leafIx = (m.leafIx + 1) % len(leafModes)
	m.flags.Leaf = leafModes[m.leafIx]
	m.cursor, m.scroll = 0, 0
	m.status = "leaf: " + m.flags.Leaf
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
