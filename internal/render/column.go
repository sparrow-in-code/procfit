package render

import (
	"fmt"
	"strings"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

// Column is a resolved output column: a header and a cell extractor. Adding a
// column is adding a descriptor here or a metric to the registry — renderers
// need no change (Open/Closed).
type Column struct {
	ID         string
	Header     string
	RightAlign bool
	// Cell renders the value for a row (without tree indentation, which the
	// table renderer applies to the target column).
	Cell func(r *query.Row) string
	// IsTarget marks the label column that receives tree indentation.
	IsTarget bool
}

// ResolveColumns turns column ids into Columns, using the metric registry for
// metric columns. Unknown ids are errors.
func ResolveColumns(reg *metrics.Registry, ids []string) ([]Column, error) {
	cols := make([]Column, 0, len(ids))
	for _, id := range ids {
		if c, ok := structuralColumn(id); ok {
			cols = append(cols, c)
			continue
		}
		desc, ok := reg.Get(id)
		if !ok {
			return nil, fmt.Errorf("unknown column %q", id)
		}
		d := desc
		cols = append(cols, Column{
			ID: id, Header: strings.ToUpper(id), RightAlign: true,
			Cell: func(r *query.Row) string { return FormatMetric(d, r.Metrics[d.ID]) },
		})
	}
	return cols, nil
}

// structuralColumns is the registry of non-metric columns. Keeping it as data
// (a map) rather than a switch keeps lookups simple and makes adding a column a
// one-line entry (Open/Closed).
var structuralColumns = map[string]Column{
	"target":  {ID: "target", Header: "TARGET", IsTarget: true, Cell: func(r *query.Row) string { return r.Label }},
	"pt":      {ID: "pt", Header: "P/T", RightAlign: true, Cell: func(r *query.Row) string { return fmt.Sprintf("%d/%d", r.Procs, r.Threads) }},
	"procs":   {ID: "procs", Header: "PROCS", RightAlign: true, Cell: func(r *query.Row) string { return fmt.Sprintf("%d", r.Procs) }},
	"threads": {ID: "threads", Header: "THREADS", RightAlign: true, Cell: func(r *query.Row) string { return fmt.Sprintf("%d", r.Threads) }},
	"pid":     {ID: "pid", Header: "PID", RightAlign: true, Cell: cellPID},
	"ppid":    {ID: "ppid", Header: "PPID", RightAlign: true, Cell: cellPPID},
	"comm":    {ID: "comm", Header: "COMM", Cell: cellComm},
	"uid":     {ID: "uid", Header: "UID", RightAlign: true, Cell: cellUID},
	"pstate":  {ID: "pstate", Header: "PSTATE", Cell: cellPState},
	"kind":    {ID: "kind", Header: "KIND", Cell: func(r *query.Row) string { return string(r.Kind) }},
}

func structuralColumn(id string) (Column, bool) {
	c, ok := structuralColumns[id]
	return c, ok
}

func cellPPID(r *query.Row) string {
	if r.Process != nil {
		return fmt.Sprintf("%d", r.Process.PPID)
	}
	return ""
}

func cellComm(r *query.Row) string {
	if r.Process != nil {
		return r.Process.Comm
	}
	return ""
}

func cellUID(r *query.Row) string {
	if r.Process != nil {
		return fmt.Sprintf("%d", r.Process.UID)
	}
	return ""
}

func cellPID(r *query.Row) string {
	if r.Process != nil {
		return fmt.Sprintf("%d", r.Process.PID)
	}
	if r.Thread != nil {
		return fmt.Sprintf("%d", r.Thread.ID.TID)
	}
	return ""
}

func cellPState(r *query.Row) string {
	if r.Process != nil {
		return string(pstateCode(r.Process.State))
	}
	return ""
}

func pstateCode(s model.ProcessState) rune {
	if s.Code == 0 {
		return '?'
	}
	return s.Code
}
