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

// IntervalHint returns the currently selected refresh interval. The step is
// clamped to the available range (it does not wrap).
func (m *Model) IntervalHint() time.Duration {
	i := m.intervalStep
	if i < 0 {
		i = 0
	}
	if i >= len(intervals) {
		i = len(intervals) - 1
	}
	return intervals[i]
}

// bodyHeight is the number of table body rows visible (excluding status + header).
func (m *Model) bodyHeight() int {
	h := m.height - 2
	if h < 1 {
		return 1
	}
	return h
}

// Frame renders the current view as text lines: a status bar, a header, then the
// visible slice of the (flattened) tree with the cursor marked.
func (m *Model) Frame() []string {
	lines := []string{m.statusBar(), m.header()}
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
	if m.editing {
		return truncate("filter> "+m.editBuf+"_", m.width)
	}
	gen, procs := 0, len(m.rows)
	if m.result != nil {
		gen = m.result.Generation
	}
	return truncate(fmt.Sprintf("%s  gen=%d rows=%d  interval=%s  [g]roup [s]ort [S]dir [/]filter [u]nits [r]efresh [q]uit  %s",
		meta.Name, gen, procs, m.IntervalHint(), m.status), m.width)
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

func truncate(s string, w int) string {
	if w <= 0 || len(s) <= w {
		return s
	}
	return s[:w]
}
