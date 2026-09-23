package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/netikras/procfit/internal/meta"
)

var intervals = []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second, 5 * time.Second}

// defaultIntervalIdx selects the initial refresh interval (1s).
const defaultIntervalIdx = 2

// IntervalHint returns the currently selected refresh interval: an explicit
// --interval value if set, otherwise the selected preset (clamped, no wrap).
func (m *Model) IntervalHint() time.Duration {
	if m.interval > 0 {
		return m.interval
	}
	i := m.intervalStep
	if i < 0 {
		i = 0
	}
	if i >= len(intervals) {
		i = len(intervals) - 1
	}
	return intervals[i]
}

// nearestPresetIdx returns the preset index closest to d (for snapping a custom
// interval back onto the [ / ] steps).
func nearestPresetIdx(d time.Duration) int {
	best, bestDiff := 0, time.Duration(1<<62)
	for i, iv := range intervals {
		diff := iv - d
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			best, bestDiff = i, diff
		}
	}
	return best
}

// bodyHeight is the number of table body rows visible (excluding the status bar,
// header, and the system panel when present).
func (m *Model) bodyHeight() int {
	h := m.height - 2 - len(m.hostPanelLines())
	if h < 1 {
		return 1
	}
	return h
}

// hostPanelLines returns the system-metrics panel — a blank separator below the
// status bar, the host matrix, then a blank line before the table header — or nil
// when the view has no host metrics.
func (m *Model) hostPanelLines() []string {
	if len(m.hostLines) == 0 {
		return nil
	}
	out := make([]string, 0, len(m.hostLines)+2)
	out = append(out, "") // separator below the status bar
	for _, ln := range m.hostLines {
		out = append(out, truncate(ln, m.width))
	}
	return append(out, "") // blank line before the table header
}

// Frame renders the current view as text lines: a status bar, a header, then the
// visible slice of the (flattened) tree with the cursor marked.
func (m *Model) Frame() []string {
	if m.help {
		return m.helpFrame()
	}
	if m.colPicker {
		return m.colPickerFrame()
	}
	if m.confirming && m.previewing {
		return m.previewFrame()
	}
	if m.detail {
		return m.detailFrame()
	}
	lines := []string{m.statusBar()}
	lines = append(lines, m.hostPanelLines()...) // system metrics panel (host-scoped), when present
	lines = append(lines, m.header())
	body := m.bodyHeight()
	for i := 0; i < body; i++ {
		idx := m.scroll + i
		if idx >= len(m.rows) {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, m.rowLine(idx))
	}
	return lines
}

func (m *Model) statusBar() string {
	if m.confirming {
		return truncate(fmt.Sprintf("CONFIRM %s %d proc(s) [%s]?  %s   [y]es [n]o [v]iew",
			m.pending.Kind.Verb(), len(m.pending.PIDs), m.pending.Label, firstLine(m.preview)), m.width)
	}
	if m.niceEditing {
		return truncate(fmt.Sprintf("nice %d proc(s) [%s]> %s_   %s",
			len(m.pending.PIDs), m.pending.Label, m.niceBuf, m.status), m.width)
	}
	if m.editing {
		// Show the edit buffer plus, in priority order: the live autocomplete
		// suggestions (columns/operators), else m.status (the entry hint or a
		// validation error on a rejected apply) — so the filter never feels inert.
		line := "filter> " + withCursor(m.editBuf, m.editPos)
		if hint := m.suggestHint(); hint != "" {
			line += "   " + hint
		} else if m.status != "" {
			line += "   " + m.status
		}
		return truncate(line, m.width)
	}

	return truncate(m.slimBar(), m.width)
}

// slimBar is the steady-state status line: identity, gen/rows, interval, the
// essential nav hint, and pointers to help + quit — then the status outlet. The
// full keymap lives in the `?` help overlay so this line stops growing as keys
// are added.
func (m *Model) slimBar() string {
	gen, procs := 0, len(m.rows)
	if m.result != nil {
		gen = m.result.Generation
	}
	refresh := "interval=" + m.IntervalHint().String()
	if m.paused {
		refresh = "PAUSED"
	}
	name := meta.Name
	if m.panel == PanelManaged {
		name += " MANAGED"
	}
	return fmt.Sprintf("%s  gen=%d rows=%d  %s   [↑↓ enter]nav  [?]help [q]uit   %s",
		name, gen, procs, refresh, m.status)
}

func (m *Model) header() string {
	var b strings.Builder
	b.WriteString("  ") // align with the row cursor marker
	for i, c := range m.cols {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(pad(c.Header, m.widthOf(i), c.RightAlign))
	}
	return truncate(strings.TrimRight(b.String(), " "), m.width)
}

func (m *Model) rowLine(idx int) string {
	fr := m.rows[idx]
	var b strings.Builder
	if idx == m.cursor {
		b.WriteString("> ")
	} else {
		b.WriteString("  ")
	}
	for i, c := range m.cols {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(pad(m.cellText(c, fr), m.widthOf(i), c.RightAlign))
	}
	return truncate(strings.TrimRight(b.String(), " "), m.width)
}

func (m *Model) widthOf(i int) int {
	if i < len(m.colWidths) {
		return m.colWidths[i]
	}
	return 0
}

// pad left- or right-justifies s to width w.
func pad(s string, w int, right bool) string {
	if len(s) >= w {
		return s
	}
	fill := strings.Repeat(" ", w-len(s))
	if right {
		return fill + s
	}
	return s + fill
}

// SelectedRow returns the row under the cursor, if any.
func (m *Model) SelectedRow() (depth int, label string, ok bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return 0, "", false
	}
	fr := m.rows[m.cursor]
	return fr.depth, fr.row.Label, true
}

// CLIString returns the equivalent `procfit ps` invocation for the current view,
// satisfying the Phase 3 exit criterion (every TUI query is reproducible).
func (m *Model) CLIString() string {
	parts := []string{meta.Name, "ps"}
	if m.flags.GroupBy != "" {
		parts = append(parts, "--group-by", m.flags.GroupBy)
	}
	if m.flags.Leaf != "" {
		parts = append(parts, "--leaf", m.flags.Leaf)
	}
	if m.flags.Sort != "" {
		parts = append(parts, "--sort", m.flags.Sort)
	}
	if m.flags.Columns != "" {
		parts = append(parts, "--columns", m.flags.Columns)
	}
	if m.flags.Select != "" {
		parts = append(parts, "--select", quoteArg(m.flags.Select))
	}
	if m.flags.Having != "" {
		parts = append(parts, "--having", quoteArg(m.flags.Having))
	}
	return strings.Join(parts, " ")
}

func quoteArg(s string) string { return "'" + s + "'" }

// firstLine returns the first line of s (the summary of a multi-line preview).
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// previewFrame renders the full per-pid control forecast as a scrollable-style
// overlay (the confirmation gate's [v]iew).
func (m *Model) previewFrame() []string {
	lines := []string{truncate(meta.Name+"  FORECAST — any key returns to confirm", m.width)}
	for _, ln := range strings.Split(m.preview, "\n") {
		lines = append(lines, truncate(ln, m.width))
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	return lines
}

// withCursor renders the edit buffer with a visible caret at pos (rune index).
func withCursor(s string, pos int) string {
	r := []rune(s)
	if pos < 0 {
		pos = 0
	}
	if pos > len(r) {
		pos = len(r)
	}
	return string(r[:pos]) + "▏" + string(r[pos:])
}

func truncate(s string, w int) string {
	if w <= 0 || len(s) <= w {
		return s
	}
	return s[:w]
}
