// Package tui is the interactive explorer. All view state and key handling live
// in Model, which is headless and unit-testable; the tcell driver (tui.go) only
// pumps events and paints Model.Frame(). This keeps the terminal-framework choice
// isolated and reversible (DEVELOPMENT.md §2.4).
package tui

import (
	"strings"
	"time"

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
}

// groupPresets are the `g`-cycle groupings (group-by only). The leaf mode is an
// independent control (`t`) so a chosen --leaf (process/thread/none) survives
// regrouping. "none" is the flat, ungrouped view.
var groupPresets = []groupPreset{
	{"ungrouped", "none"},
	{"by comm", "comm"},
	{"by name", "name"},
	{"by user", "user"},
	{"by app", "app"},
	{"by namespace-set", "namespace-set,comm"},
}

// leafModes are the `t`-cycle terminal-row modes under groups.
var leafModes = []string{"process", "thread", "none"}

// Model holds all interactive state.
type Model struct {
	flags        queryspec.Flags
	result       *query.Result
	cols         []render.Column
	colWidths    []int
	rows         []flatRow
	width        int
	height       int
	scroll       int
	cursor       int
	groupIx      int
	leafIx       int
	sortIx       int
	sortDsc      bool
	intervalStep int
	interval     time.Duration // explicit --interval; 0 = use presets
	status       string
	quit         bool
	dirty        bool // a re-query is needed
	paused       bool // auto-refresh (ticker) suspended
	editing      bool // filter-edit mode active
	editBuf      string
	editPos      int             // cursor position (rune index) within editBuf
	history      []string        // applied filter expressions, oldest first
	histIdx      int             // browse position into history (== len(history) means "live")
	collapsed    map[string]bool // explicit per-group fold overrides (path -> folded)
	foldDefault  bool            // default fold state for groups with no override (fold-all / --collapse-groups)
	hasGroups    bool            // the current view actually has a tree (not a flat list)

	// control (throttling) state; control is nil when the driver supplies no
	// controller (observation-only).
	control     ControlFunc
	confirming  bool           // a control confirmation gate is open
	previewing  bool           // the full per-pid forecast overlay is open
	niceEditing bool           // entering a nice value
	niceBuf     string         // the nice value being typed
	pending     ControlRequest // the action awaiting a value/confirmation
	preview     string         // dry-run preview shown in the confirmation gate

	detail    bool // inspect overlay open
	historyFn HistoryFunc

	// managed panel: renders the SAME tree as the browser, fed by the managed
	// data source; only the legend, the drop action, and the data differ.
	panel  Panel
	hasMgr bool     // a managed data source is available (enables Tab)
	dropFn DropFunc // unmanage the selected row's target(s)
}

// Panel reports which view is active (browser vs managed), so the driver can pick
// the matching data source.
func (m *Model) Panel() Panel { return m.panel }

// SetManaged enables the managed panel: hasSource toggles Tab availability, drop
// unmanages the selected row's target(s).
func (m *Model) SetManaged(hasSource bool, drop DropFunc) {
	m.hasMgr = hasSource
	m.dropFn = drop
}

// SetControl installs the control callback (enables throttling keys). Nil keeps
// the explorer observation-only.
func (m *Model) SetControl(fn ControlFunc) { m.control = fn }

type flatRow struct {
	row       *query.Row
	depth     int
	path      string // stable identity across refreshes (ancestor keys joined)
	hasKids   bool   // an expandable (sub)group
	collapsed bool   // currently folded shut
}

// NewModel builds a model with initial flags and a 1s default refresh interval.
func NewModel(flags queryspec.Flags) *Model {
	m := &Model{flags: flags, height: 24, width: 100, intervalStep: defaultIntervalIdx, collapsed: map[string]bool{}}
	m.foldDefault = flags.CollapseGroups                // --collapse-groups / config: start folded
	m.groupIx = indexOf(groupByLabels(), flags.GroupBy) // keep g-cycle in sync with launch flags
	m.leafIx = indexOf(leafModes, leafOrDefault(flags.Leaf))
	if flags.Sort != "" {
		m.status = "sorted by " + flags.Sort
	}
	return m
}

func groupByLabels() []string {
	out := make([]string, len(groupPresets))
	for i, p := range groupPresets {
		out[i] = p.groupBy
	}
	return out
}

func leafOrDefault(leaf string) string {
	if leaf == "" {
		return "process"
	}
	return leaf
}

// indexOf returns the position of v in xs, or 0 when absent (a safe default that
// keeps cycling coherent even if launch flags used a value with no preset).
func indexOf(xs []string, v string) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return 0
}

// Flags returns the current query flags (for the sampler).
func (m *Model) Flags() queryspec.Flags { return m.flags }

// SetInterval sets an explicit starting refresh interval (from --interval). The
// [ / ] keys still snap back to the preset steps from the nearest value.
func (m *Model) SetInterval(d time.Duration) {
	if d > 0 {
		m.interval = d
		m.intervalStep = nearestPresetIdx(d)
	}
}

// Quit reports whether the user asked to exit.
func (m *Model) Quit() bool { return m.quit }

// Paused reports whether auto-refresh is suspended. The driver honours this on
// the ticker tick; explicit actions (refresh, sort, filter, …) still re-query.
func (m *Model) Paused() bool { return m.paused }

// RefreshSuspended reports whether ticker-driven auto-refresh must hold off:
// either the user paused, or a modal gate is open (a control confirmation, its
// forecast overlay, or the nice-value prompt). Freezing the view during a
// control decision keeps the row list from reordering under the user mid-gate.
// The action itself is already pinned to the pids captured when it began, so
// this is about a steady view, not correctness — a reorder can never retarget a
// confirmed action.
func (m *Model) RefreshSuspended() bool {
	return m.paused || m.confirming || m.previewing || m.niceEditing
}

// Dirty reports (and clears) whether a re-query is needed.
func (m *Model) Dirty() bool {
	d := m.dirty
	m.dirty = false
	return d
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

// SetResult stores the latest query result and its columns, recomputing the
// per-column display widths so the header and rows stay aligned.
func (m *Model) SetResult(res *query.Result, cols []render.Column) {
	m.result = res
	m.cols = cols
	if len(m.cols) > 0 && !m.cols[safeIdx(m.sortIx, len(m.cols))].Sortable {
		m.sortIx = m.firstSortable() // keep the sort key on a sortable displayed column
	}
	m.rebuildRows()
}

// rebuildRows re-derives the visible flat row list from the current result and
// collapse state, then recomputes widths and re-clamps cursor/scroll. Called on
// a new result AND after an expand/collapse (a view-only change, no re-query).
func (m *Model) rebuildRows() {
	m.rows = m.flattenTree()
	m.colWidths = m.computeWidths()
	if m.cursor >= len(m.rows) {
		m.cursor = maxInt(0, len(m.rows)-1)
	}
	m.clampScroll()
}

func safeIdx(i, n int) int {
	if n == 0 {
		return 0
	}
	if i < 0 || i >= n {
		return 0
	}
	return i
}

// firstSortable returns the index of the first sortable column, or 0.
func (m *Model) firstSortable() int {
	for i, c := range m.cols {
		if c.Sortable {
			return i
		}
	}
	return 0
}

// filterableFields is the set of displayed columns usable in a `having` filter,
// so the interactive filter only offers properties that are actually shown.
func (m *Model) filterableFields() map[string]bool {
	out := map[string]bool{}
	for _, c := range m.cols {
		if c.Filterable {
			out[c.ID] = true
		}
	}
	return out
}

// computeWidths returns the display width of each column: the max of the header
// and every row's cell (target cells include their tree indentation).
func (m *Model) computeWidths() []int {
	w := make([]int, len(m.cols))
	for i, c := range m.cols {
		w[i] = len(c.Header)
	}
	for _, fr := range m.rows {
		for i, c := range m.cols {
			n := len(m.cellText(c, fr))
			if n > w[i] {
				w[i] = n
			}
		}
	}
	return w
}

// cellText renders a column's cell for a flat row, applying tree indentation to
// the target column.
func (m *Model) cellText(c render.Column, fr flatRow) string {
	cell := c.Cell(fr.row)
	if c.IsTarget {
		// Only draw indentation + fold markers when the view is actually a tree.
		// A flat process list (no grouping) stays flush-left, not pointlessly
		// indented.
		prefix := ""
		if m.hasGroups {
			prefix = indent(fr.depth) + treeMarker(fr)
		}
		cell = render.TruncateCell(prefix+cell, m.flags.TargetWidth)
	}
	return cell
}

// treeMarker shows whether a row is an expandable (sub)group and its fold state,
// padded so groups and leaves at the same depth line up: "[-] " expanded,
// "[+] " collapsed, "    " leaf.
func treeMarker(fr flatRow) string {
	switch {
	case !fr.hasKids:
		return "    "
	case fr.collapsed:
		return "[+] "
	default:
		return "[-] "
	}
}

func indent(depth int) string {
	if depth <= 0 {
		return ""
	}
	return strings.Repeat("  ", depth)
}

// isCollapsed reports whether the group at path is folded: an explicit user
// override (Enter/←/→) wins; otherwise the current default applies (set by
// fold-all `c`/`C` and the --collapse-groups launch flag), so groups that appear
// after a fold-all — or on regroup — follow it too.
func (m *Model) isCollapsed(path string) bool {
	if v, ok := m.collapsed[path]; ok {
		return v
	}
	return m.foldDefault
}

// foldAll folds (or unfolds) every group at once by flipping the default and
// dropping per-group overrides, so the whole tree — including groups not yet
// materialised — follows. A view-only change (no re-query), so it works paused.
func (m *Model) foldAll(collapse bool) {
	if !m.hasGroups {
		m.status = "flat list — press g to group first"
		return
	}
	m.foldDefault = collapse
	m.collapsed = map[string]bool{}
	m.status = "all groups expanded"
	if collapse {
		m.status = "all groups collapsed"
	}
	m.rebuildRows()
}

// flattenTree walks the result tree depth-first, giving each row a stable path
// (ancestor keys joined) and skipping the children of collapsed groups, so the
// visible list reflects the fold state.
func (m *Model) flattenTree() []flatRow {
	if m.result == nil {
		return nil
	}
	var out []flatRow
	m.hasGroups = false
	var walk func(rows []*query.Row, depth int, parent string)
	walk = func(rows []*query.Row, depth int, parent string) {
		for _, r := range rows {
			path := parent + "\x1f" + r.Key
			kids := len(r.Sub) > 0
			if r.Kind == query.RowGroup || r.Kind == query.RowHost {
				m.hasGroups = true
			}
			folded := kids && m.isCollapsed(path)
			out = append(out, flatRow{row: r, depth: depth, path: path, hasKids: kids, collapsed: folded})
			if kids && !folded {
				walk(r.Sub, depth+1, path)
			}
		}
	}
	walk(m.result.Rows, 0, "")
	return out
}
