package render

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

// StructuralColumns returns the built-in non-metric (identity/state) columns in a
// stable id order. It is the single source of truth for enumerating and
// validating structural columns (e.g. `procfit metrics`), so it never drifts.
func StructuralColumns() []Column {
	ids := make([]string, 0, len(structuralColumns))
	for id := range structuralColumns {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Column, 0, len(ids))
	for _, id := range ids {
		out = append(out, structuralColumns[id])
	}
	return out
}

// Column is a resolved output column: a header and a cell extractor. Adding a
// column is adding a descriptor here or a metric to the registry — renderers
// need no change (Open/Closed).
type Column struct {
	ID         string
	Header     string
	RightAlign bool
	// Cell renders the human value for a row (without tree indentation, which the
	// table renderer applies to the target column).
	Cell func(r *query.Row) string
	// Machine renders the stable machine value (raw numbers, no unit scaling,
	// empty string when unavailable) for CSV/NDJSON output.
	Machine func(r *query.Row) string
	// IsTarget marks the label column that receives tree indentation.
	IsTarget bool
	// Sortable / Filterable report whether the query engine can sort by / filter
	// (having) on this column's field. The TUI uses these so interactive sort and
	// filter only offer properties that are actually displayed AND usable.
	Sortable   bool
	Filterable bool
	// Groupable reports whether this column's id is also a group-by dimension
	// (a categorical/identity field, not a continuous metric). The TUI offers
	// grouping by any displayed groupable column.
	Groupable bool
	// Desc is a one-line description for the field catalog (`procfit metrics`).
	Desc string
}

// ResolveColumns turns column ids into Columns with human-readable metric
// formatting (K/M/G). Unknown ids are errors.
func ResolveColumns(reg *metrics.Registry, ids []string) ([]Column, error) {
	return ResolveColumnsMode(reg, ids, true)
}

// ResolveColumnsMode is like ResolveColumns but chooses metric cell formatting:
// human=true scales units (K/M/G); human=false prints raw bytes/numbers (like
// `free` without -h). Machine output (csv/ndjson) is always raw regardless.
func ResolveColumnsMode(reg *metrics.Registry, ids []string, human bool) ([]Column, error) {
	cols := make([]Column, 0, len(ids))
	for _, id := range ids {
		if c, ok := structuralColumn(id); ok {
			if c.Machine == nil {
				c.Machine = c.Cell // structural machine value == display value
			}
			cols = append(cols, c)
			continue
		}
		desc, ok := reg.Get(id)
		if !ok {
			return nil, fmt.Errorf("unknown column %q", id)
		}
		d := desc
		cell := func(r *query.Row) string { return FormatMetricRaw(d, r.Metrics[d.ID]) }
		if human {
			cell = func(r *query.Row) string { return FormatMetric(d, r.Metrics[d.ID]) }
		}
		cols = append(cols, Column{
			ID: id, Header: strings.ToUpper(id), RightAlign: true,
			Cell:       cell,
			Machine:    func(r *query.Row) string { return machineMetric(r.Metrics[d.ID]) },
			Sortable:   true, // every metric is sortable and usable in `having`
			Filterable: true,
		})
	}
	return cols, nil
}

// machineMetric renders a metric's raw value for machine formats: the unscaled
// float, or an empty field when unavailable (never a fabricated zero).
func machineMetric(v model.MetricValue) string {
	if val, ok := v.Get(); ok {
		return strconv.FormatFloat(val, 'f', -1, 64)
	}
	return ""
}

// structuralColumns is the registry of non-metric columns. Keeping it as data
// (a map) rather than a switch keeps lookups simple and makes adding a column a
// one-line entry (Open/Closed).
var structuralColumns = map[string]Column{
	"target":   {ID: "target", Header: "TARGET", IsTarget: true, Sortable: true, Filterable: true, Desc: "the row's group/process label (the tree target)", Cell: func(r *query.Row) string { return r.Label }},
	"pt":       {ID: "pt", Header: "P/T", RightAlign: true, Desc: "process/thread counts under the row", Cell: func(r *query.Row) string { return fmt.Sprintf("%d/%d", r.Procs, r.Threads) }},
	"procs":    {ID: "procs", Header: "PROCS", RightAlign: true, Sortable: true, Filterable: true, Desc: "number of processes in the row", Cell: func(r *query.Row) string { return fmt.Sprintf("%d", r.Procs) }},
	"threads":  {ID: "threads", Header: "THREADS", RightAlign: true, Sortable: true, Filterable: true, Desc: "number of threads in the row", Cell: func(r *query.Row) string { return fmt.Sprintf("%d", r.Threads) }},
	"pid":      {ID: "pid", Header: "PID", RightAlign: true, Sortable: true, Desc: "process id", Cell: cellPID},
	"tid":      {ID: "tid", Header: "TID", RightAlign: true, Sortable: true, Desc: "thread id (thread leaves)", Cell: cellTID},
	"ppid":     {ID: "ppid", Header: "PPID", RightAlign: true, Groupable: true, Desc: "parent process id", Cell: cellPPID},
	"comm":     {ID: "comm", Header: "COMM", Sortable: true, Filterable: true, Groupable: true, Desc: "kernel comm (short command name)", Cell: cellComm},
	"name":     {ID: "name", Header: "NAME", Sortable: true, Filterable: true, Groupable: true, Desc: "display name (best-effort process name)", Cell: cellName},
	"uid":      {ID: "uid", Header: "UID", RightAlign: true, Groupable: true, Desc: "real user id", Cell: cellUID},
	"user":     {ID: "user", Header: "USER", Groupable: true, Desc: "resolved user name", Cell: cellUser},
	"pstate":   {ID: "pstate", Header: "PSTATE", Sortable: true, Filterable: true, Groupable: true, Desc: "process state (R/S/D/Z/I/T/…)", Cell: cellPState},
	"kind":     {ID: "kind", Header: "KIND", Filterable: true, Desc: "row kind (group/process/thread)", Cell: func(r *query.Row) string { return string(r.Kind) }},
	"cmdline":  {ID: "cmdline", Header: "CMDLINE", Desc: "full command line", Cell: cellCmdline},
	"wchan":    {ID: "wchan", Header: "WCHAN", Sortable: true, Filterable: true, Groupable: true, Desc: "kernel symbol a blocked task sleeps in (names a D-state cause)", Cell: cellWchan},
	"hostname": {ID: "hostname", Header: "HOSTNAME", Sortable: true, Filterable: true, Groupable: true, Desc: "process HOSTNAME env var, else the host hostname (poor-man's container name)", Cell: cellHostname},
}

// cellHostname renders a process's HOSTNAME env (or the host fallback).
func cellHostname(r *query.Row) string {
	if r.Process != nil {
		return r.Process.Hostname
	}
	return ""
}

// cellWchan renders the kernel symbol a blocked process is sleeping in.
func cellWchan(r *query.Row) string {
	if r.Process != nil {
		return r.Process.Wchan
	}
	return ""
}

// cellCmdline renders the full command line of a process leaf (the distinguishing
// detail that the compact NAME/target column intentionally drops).
func cellCmdline(r *query.Row) string {
	if r.Process != nil {
		return strings.Join(r.Process.Cmdline, " ")
	}
	return ""
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

func cellName(r *query.Row) string {
	if r.Process != nil {
		return r.Process.DisplayName()
	}
	return ""
}

func cellUID(r *query.Row) string {
	if r.Process != nil {
		return fmt.Sprintf("%d", r.Process.UID)
	}
	return ""
}

func cellUser(r *query.Row) string {
	if r.Process != nil {
		return r.Process.User
	}
	return ""
}

// cellPID renders the owning process id: a process leaf's PID, or a thread
// leaf's owner PID. Group rows have no PID.
func cellPID(r *query.Row) string {
	if r.Process != nil {
		return fmt.Sprintf("%d", r.Process.PID)
	}
	if r.Thread != nil {
		return fmt.Sprintf("%d", r.Thread.ID.Process.PID)
	}
	return ""
}

// cellTID renders a thread leaf's thread id; blank for processes and groups, so
// PID and TID read as distinct identifier columns.
func cellTID(r *query.Row) string {
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
