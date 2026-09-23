package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/render"
)

// hostMatrixRows is the fixed number of rows in the system-metrics matrix; cells
// fill top-to-bottom then left-to-right (column-major).
const hostMatrixRows = 3

// isHostMetric reports whether a metric id is host-scoped (rendered in the system
// section, never as a per-process column).
func (a *assembly) isHostMetric(id string) bool {
	d, ok := a.reg.Get(id)
	return ok && d.Scope == metrics.ScopeHost
}

// processColumns drops host-scoped metric ids from a column list: those belong to
// the system section, not the per-process table.
func (a *assembly) processColumns(cols []string) []string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if !a.isHostMetric(c) {
			out = append(out, c)
		}
	}
	return out
}

// hostSection renders the banner + aligned system-metrics matrix for a result,
// or nil when the query produced no host metric (then the caller adds no section
// and the output is unchanged). human selects K/M/G-scaled values.
func (a *assembly) hostSection(res *query.Result, human bool) []string {
	if len(res.HostMetrics) == 0 {
		return nil
	}
	// Sections are blank-line separated: banner, then the system-metrics matrix.
	lines := []string{hostBanner(res), ""}
	lines = append(lines, a.hostMatrix(res.HostMetrics, human)...)
	return lines
}

// hostBanner is the one-line context header above the system section.
func hostBanner(res *query.Result) string {
	b := fmt.Sprintf("system  ·  %d process", totalProcs(res))
	if totalProcs(res) != 1 {
		b += "es"
	}
	if !res.WallTime.IsZero() {
		b += "  ·  " + res.WallTime.Format("2006-01-02 15:04:05")
	}
	return b
}

func totalProcs(res *query.Result) int {
	n := 0
	for _, r := range res.Rows {
		n += r.Procs
	}
	return n
}

// hostMatrix formats the host metrics as an aligned key/value matrix: metrics
// sorted alphabetically and laid out column-major over hostMatrixRows rows. In
// each column the value is single-spaced past the column's longest key (so values
// align), and columns are separated by four spaces after the value; a value with
// spaces is quoted.
func (a *assembly) hostMatrix(hm map[model.MetricID]model.MetricValue, human bool) []string {
	ids := make([]string, 0, len(hm))
	for id := range hm {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	vals := make([]string, len(ids))
	for i, id := range ids {
		vals[i] = quoteIfSpace(a.hostValue(id, hm[model.MetricID(id)], human))
	}
	return layoutKV(ids, vals, hostMatrixRows)
}

// hostValue formats one host metric value (human or raw), or its unavailable
// placeholder.
func (a *assembly) hostValue(id string, v model.MetricValue, human bool) string {
	d, ok := a.reg.Get(id)
	if !ok {
		return "-"
	}
	if human {
		return render.FormatMetric(d, v)
	}
	return render.FormatMetricRaw(d, v)
}

func quoteIfSpace(s string) string {
	if strings.ContainsAny(s, " \t") {
		return strconv.Quote(s)
	}
	return s
}

// kvGap is the number of spaces between a value and the next column's key.
const kvGap = 4

// layoutKV renders key/value pairs into `rows` rows, filled column-major
// (top-bottom, then left-right). Within each column the key is padded to the
// column's widest key so every value starts one space past it, and each value is
// padded to the column's widest value so the next column's keys line up after a
// four-space gap. Trailing padding is trimmed.
func layoutKV(keys, vals []string, rows int) []string {
	if len(keys) == 0 || rows < 1 {
		return nil
	}
	cols := (len(keys) + rows - 1) / rows
	keyW, valW := make([]int, cols), make([]int, cols)
	for i := range keys {
		c := i / rows
		if len(keys[i]) > keyW[c] {
			keyW[c] = len(keys[i])
		}
		if len(vals[i]) > valW[c] {
			valW[c] = len(vals[i])
		}
	}
	out := make([]string, 0, rows)
	for r := 0; r < rows; r++ {
		if line := kvRow(keys, vals, keyW, valW, r, rows, cols); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// kvRow renders one row of the column-major key/value grid.
func kvRow(keys, vals []string, keyW, valW []int, r, rows, cols int) string {
	var b strings.Builder
	for c := 0; c < cols; c++ {
		i := c*rows + r
		if i >= len(keys) {
			continue
		}
		b.WriteString(padRight(keys[i], keyW[c]))
		b.WriteByte(' ')
		b.WriteString(vals[i])
		if c < cols-1 {
			b.WriteString(strings.Repeat(" ", valW[c]-len(vals[i])+kvGap))
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// padRight left-justifies s to width w.
func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
