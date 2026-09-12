package query

import (
	"fmt"
	"time"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
)

// Input is the engine's snapshot-agnostic input, so query need not import the
// collect package (keeps the dependency arrow pointing inward).
type Input struct {
	Generation int
	WallTime   time.Time
	Elapsed    time.Duration
	Processes  []model.Process
}

// Engine transforms a snapshot into a grouped/aggregated/sorted result per a
// QuerySpec. It depends only on the metric and dimension registries.
type Engine struct {
	reg  *metrics.Registry
	dims *Dimensions
}

// NewEngine constructs an engine.
func NewEngine(reg *metrics.Registry, dims *Dimensions) *Engine {
	return &Engine{reg: reg, dims: dims}
}

// Build runs the full pipeline: select → group → aggregate → leaves → having →
// sort → project.
func (e *Engine) Build(in Input, spec QuerySpec) (*Result, error) {
	dims, err := e.resolveGroupBy(spec.GroupBy)
	if err != nil {
		return nil, err
	}
	leaf := spec.Leaf
	if leaf == "" {
		leaf = LeafProcess
	}

	procs := selectProcesses(in.Processes, spec.Select)

	var rows []*Row
	if len(dims) == 0 && leaf == LeafNone {
		rows = []*Row{e.hostTotal(procs, spec.Metrics)}
	} else {
		rows = e.build(procs, dims, leaf, spec.Metrics)
	}

	rows = applyHaving(rows, spec.Having)
	e.sortTree(rows, spec.Sort)
	if spec.Limit > 0 && len(rows) > spec.Limit {
		rows = rows[:spec.Limit] // top-N after sorting (RFC: filter+order already applied)
	}

	return &Result{
		Generation: in.Generation,
		WallTime:   in.WallTime,
		Elapsed:    in.Elapsed,
		Columns:    spec.Columns,
		Rows:       rows,
	}, nil
}

// resolveGroupBy validates and resolves group-by ids to dimensions. "none" must
// be the only value; duplicates are rejected (RFC §8.3).
func (e *Engine) resolveGroupBy(ids []string) ([]Dimension, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) == 1 && ids[0] == "none" {
		return nil, nil
	}
	seen := map[string]bool{}
	dims := make([]Dimension, 0, len(ids))
	for _, id := range ids {
		if id == "none" {
			return nil, fmt.Errorf("group-by 'none' must be the only value")
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate group-by dimension %q", id)
		}
		seen[id] = true
		d, ok := e.dims.Get(id)
		if !ok {
			return nil, fmt.Errorf("unknown group-by dimension %q", id)
		}
		dims = append(dims, d)
	}
	return dims, nil
}

func selectProcesses(all []model.Process, pred EntityPredicate) []*model.Process {
	out := make([]*model.Process, 0, len(all))
	for i := range all {
		p := &all[i]
		if pred == nil || pred.EvalEntity(p) {
			out = append(out, p)
		}
	}
	return out
}

// build recurses through the dimension list, then attaches leaves.
func (e *Engine) build(procs []*model.Process, dims []Dimension, leaf LeafMode, ids []model.MetricID) []*Row {
	if len(dims) == 0 {
		return e.makeLeaves(procs, leaf, ids)
	}
	dim := dims[0]
	order, buckets := bucketBy(procs, dim)
	rows := make([]*Row, 0, len(order))
	for _, key := range order {
		bp := buckets[key]
		row := &Row{Kind: RowGroup, DimensionID: dim.ID, Key: key, Label: bucketLabel(dim, bp, key)}
		row.Sub = e.build(bp, dims[1:], leaf, ids)
		e.fillGroup(row, bp, ids)
		rows = append(rows, row)
	}
	return rows
}

func (e *Engine) makeLeaves(procs []*model.Process, leaf LeafMode, ids []model.MetricID) []*Row {
	switch leaf {
	case LeafNone:
		return nil
	case LeafThread:
		return threadLeaves(procs, ids)
	default: // process
		return processLeaves(procs, ids)
	}
}

func processLeaves(procs []*model.Process, ids []model.MetricID) []*Row {
	rows := make([]*Row, 0, len(procs))
	for _, p := range procs {
		rows = append(rows, &Row{
			Kind: RowProcess, Key: p.ID.Key(), Label: leafLabel(p),
			Procs: 1, Threads: procThreadCount(p), Leaves: 1,
			Metrics: projectMetrics(p.Metrics, ids), Process: p,
		})
	}
	return rows
}

func threadLeaves(procs []*model.Process, ids []model.MetricID) []*Row {
	var rows []*Row
	for _, p := range procs {
		for i := range p.Threads {
			th := &p.Threads[i]
			rows = append(rows, &Row{
				Kind: RowThread, Key: th.ID.Key(), Label: th.Comm,
				Procs: 0, Threads: 1, Leaves: 1,
				Metrics: projectMetrics(th.Metrics, ids), Thread: th,
			})
		}
	}
	return rows
}

func (e *Engine) fillGroup(row *Row, procs []*model.Process, ids []model.MetricID) {
	row.Children = len(row.Sub)
	row.Procs = len(procs)
	threads := 0
	for _, p := range procs {
		threads += procThreadCount(p)
	}
	row.Threads = threads
	row.Leaves = countLeaves(row)
	row.Metrics = aggregateMetrics(e.reg, procs, ids)
}

func (e *Engine) hostTotal(procs []*model.Process, ids []model.MetricID) *Row {
	row := &Row{Kind: RowHost, Key: "host", Label: "HOST"}
	e.fillGroup(row, procs, ids)
	return row
}

func countLeaves(r *Row) int {
	if r.Kind == RowProcess || r.Kind == RowThread {
		return 1
	}
	n := 0
	for _, c := range r.Sub {
		n += countLeaves(c)
	}
	return n
}
