package query

import (
	"time"

	"github.com/netikras/procfit/internal/model"
)

// LeafMode selects the terminal row kind beneath groups (RFC §8.3), independent
// of grouping.
type LeafMode string

const (
	LeafProcess LeafMode = "process"
	LeafThread  LeafMode = "thread"
	LeafNone    LeafMode = "none"
)

// SortKey is one sort dimension (RFC §8.4).
type SortKey struct {
	Field      string
	Descending bool
}

// EntityPredicate filters individual entities before grouping (the `select`
// stage, RFC §10.1). A nil predicate passes everything.
type EntityPredicate interface {
	EvalEntity(p *model.Process) bool
}

// RowPredicate filters aggregated rows after grouping (the `having` stage). A
// nil predicate passes everything.
type RowPredicate interface {
	EvalRow(r *Row) bool
}

// QuerySpec is the renderer- and transport-agnostic description of a query (RFC
// §25). It is the stable contract between CLI/TUI/daemon and the engine.
type QuerySpec struct {
	Metrics []model.MetricID
	Select  EntityPredicate
	GroupBy []string
	Leaf    LeafMode
	Having  RowPredicate
	Sort    []SortKey
	Columns []string
}

// RowKind identifies what a row represents.
type RowKind string

const (
	RowHost    RowKind = "host"
	RowGroup   RowKind = "group"
	RowProcess RowKind = "process"
	RowThread  RowKind = "thread"
)

// Row is a node in the result tree.
type Row struct {
	Kind        RowKind
	DimensionID string
	Key         string
	Label       string

	Children int
	Procs    int
	Threads  int
	Leaves   int

	Metrics map[model.MetricID]model.MetricValue

	// Process is set for process leaf rows; Thread for thread leaf rows.
	Process *model.Process
	Thread  *model.Thread

	Sub []*Row
}

// Result is the output of the engine for one snapshot.
type Result struct {
	Generation int
	WallTime   time.Time
	Elapsed    time.Duration
	Columns    []string
	Rows       []*Row
}
