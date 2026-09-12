// Package csv renders a query.Result as CSV for stable machine consumption
// (RFC §7.3). Metric cells use raw unscaled values; unavailable metrics are
// empty fields (never zero). A row's tree depth is emitted as a column so the
// hierarchy is recoverable without parsing indentation.
package csv

import (
	"encoding/csv"
	"io"
	"strconv"

	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/render"
)

// Render writes a full CSV document (header + rows).
func Render(w io.Writer, res *query.Result, cols []render.Column) error {
	cw := csv.NewWriter(w)
	if err := writeHeader(cw, cols); err != nil {
		return err
	}
	if err := writeRows(cw, res, cols); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// WriteHeader writes just the CSV header (for streaming: emit once).
func WriteHeader(w io.Writer, cols []render.Column) error {
	cw := csv.NewWriter(w)
	if err := writeHeader(cw, cols); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// WriteRows writes just the data rows for one result (for streaming: per tick).
func WriteRows(w io.Writer, res *query.Result, cols []render.Column) error {
	cw := csv.NewWriter(w)
	if err := writeRows(cw, res, cols); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

func writeHeader(cw *csv.Writer, cols []render.Column) error {
	header := []string{"time", "depth", "kind"}
	for _, c := range cols {
		header = append(header, c.ID)
	}
	return cw.Write(header)
}

func writeRows(cw *csv.Writer, res *query.Result, cols []render.Column) error {
	ts := res.WallTime.UTC().Format("2006-01-02T15:04:05Z07:00")
	for _, fr := range flatten(res.Rows, 0) {
		rec := []string{ts, strconv.Itoa(fr.depth), string(fr.row.Kind)}
		for _, c := range cols {
			rec = append(rec, c.Machine(fr.row))
		}
		if err := cw.Write(rec); err != nil {
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
