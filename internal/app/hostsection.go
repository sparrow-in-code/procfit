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

// hostMatrix formats the host metrics as a column-aligned matrix (like `column
// -t`): cells `id=value`, sorted alphabetically, laid out column-major over
// hostMatrixRows rows; a value containing spaces is quoted.
func (a *assembly) hostMatrix(hm map[model.MetricID]model.MetricValue, human bool) []string {
	ids := make([]string, 0, len(hm))
	for id := range hm {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	cells := make([]string, len(ids))
	for i, id := range ids {
		cells[i] = id + "=" + quoteIfSpace(a.hostValue(id, hm[model.MetricID(id)], human))
	}
	return layoutMatrix(cells, hostMatrixRows)
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

// layoutMatrix arranges cells into `rows` rows, filled column-major (top-bottom,
// then left-right), padding each column to its widest cell (two-space gap).
func layoutMatrix(cells []string, rows int) []string {
	if len(cells) == 0 || rows < 1 {
		return nil
	}
	cols := (len(cells) + rows - 1) / rows
	grid := make([][]string, rows)
	for r := range grid {
		grid[r] = make([]string, cols)
	}
	for i, cell := range cells {
		grid[i%rows][i/rows] = cell
	}
	widths := colWidths(grid, cols)
	out := make([]string, 0, rows)
	for r := 0; r < rows; r++ {
		if line := matrixRow(grid[r], widths); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// colWidths returns the widest cell in each column of the grid.
func colWidths(grid [][]string, cols int) []int {
	widths := make([]int, cols)
	for _, row := range grid {
		for c, cell := range row {
			if len(cell) > widths[c] {
				widths[c] = len(cell)
			}
		}
	}
	return widths
}

// matrixRow renders one grid row, padding each non-final cell to its column width
// plus a two-space gap; trailing padding is trimmed.
func matrixRow(cells []string, widths []int) string {
	var b strings.Builder
	for c, cell := range cells {
		if cell == "" {
			continue
		}
		b.WriteString(cell)
		if c < len(cells)-1 {
			b.WriteString(strings.Repeat(" ", widths[c]-len(cell)+2))
		}
	}
	return strings.TrimRight(b.String(), " ")
}
