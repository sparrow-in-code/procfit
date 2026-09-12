// Package table renders a query.Result as an aligned, human-readable tree table
// (RFC §18.3). It is pure presentation over the resolved columns; it performs no
// collection, filtering, grouping, or aggregation (RFC §6.1).
package table

import (
	"fmt"
	"io"
	"strings"

	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/render"
)

// Render writes the result as a table using the given resolved columns.
func Render(w io.Writer, res *query.Result, cols []render.Column) error {
	rows := flatten(res.Rows, 0)
	matrix := make([][]string, 0, len(rows)+1)

	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = c.Header
	}
	matrix = append(matrix, header)

	for _, fr := range rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cell := c.Cell(fr.row)
			if c.IsTarget {
				cell = strings.Repeat("  ", fr.depth) + cell
			}
			cells[i] = cell
		}
		matrix = append(matrix, cells)
	}

	widths := columnWidths(matrix)
	for _, row := range matrix {
		if err := writeRow(w, row, widths, cols); err != nil {
			return err
		}
	}
	return nil
}

type flatRow struct {
	row   *query.Row
	depth int
}

func flatten(rows []*query.Row, depth int) []flatRow {
	var out []flatRow
	for _, r := range rows {
		out = append(out, flatRow{row: r, depth: depth})
		out = append(out, flatten(r.Sub, depth+1)...)
	}
	return out
}

func columnWidths(matrix [][]string) []int {
	if len(matrix) == 0 {
		return nil
	}
	widths := make([]int, len(matrix[0]))
	for _, row := range matrix {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	return widths
}

func writeRow(w io.Writer, cells []string, widths []int, cols []render.Column) error {
	var b strings.Builder
	for i, cell := range cells {
		if i > 0 {
			b.WriteString("  ")
		}
		pad := widths[i] - len(cell)
		last := i == len(cells)-1
		if cols[i].RightAlign {
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString(cell)
		} else {
			b.WriteString(cell)
			if !last {
				b.WriteString(strings.Repeat(" ", pad))
			}
		}
	}
	_, err := fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	return err
}
