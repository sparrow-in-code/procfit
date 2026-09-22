package queryspec

import (
	"github.com/netikras/procfit/internal/expr"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

// EntityEnv adapts a process to expr.Env for the `select` stage: metric values
// come from the computed metric map, structural fields from the entity.
type EntityEnv struct{ P *model.Process }

// Lookup resolves a field to a Value (Missing if unknown/unavailable).
func (e EntityEnv) Lookup(field string) expr.Value {
	if v, ok := e.P.Metrics[model.MetricID(field)]; ok {
		if n, ok := v.Get(); ok {
			return expr.Num(n)
		}
		return expr.Missing
	}
	return EntityField(e.P, field)
}

var entityNumFields = map[string]func(*model.Process) float64{
	"pid":     func(p *model.Process) float64 { return float64(p.PID) },
	"ppid":    func(p *model.Process) float64 { return float64(p.PPID) },
	"pgid":    func(p *model.Process) float64 { return float64(p.PGID) },
	"sid":     func(p *model.Process) float64 { return float64(p.SID) },
	"session": func(p *model.Process) float64 { return float64(p.SID) },
	"uid":     func(p *model.Process) float64 { return float64(p.UID) },
	"gid":     func(p *model.Process) float64 { return float64(p.GID) },
}

var entityStrFields = map[string]func(*model.Process) string{
	"comm":         func(p *model.Process) string { return p.Comm },
	"name":         func(p *model.Process) string { return p.DisplayName() },
	"user":         func(p *model.Process) string { return p.User },
	"exe":          func(p *model.Process) string { return p.Exe },
	"app":          func(p *model.Process) string { return p.AppID },
	"cgroup":       func(p *model.Process) string { return p.CgroupPath },
	"systemd-unit": func(p *model.Process) string { return p.SystemdUnit },
	"container":    func(p *model.Process) string { return p.ContainerID },
	"pod":          func(p *model.Process) string { return p.PodUID },
	"state":        func(p *model.Process) string { return string(p.State.Code) },
	"pstate":       func(p *model.Process) string { return string(p.State.Code) }, // alias matching the column id
	"wchan":        func(p *model.Process) string { return p.Wchan },
}

// EntityField resolves a non-metric entity field to a Value.
func EntityField(p *model.Process, field string) expr.Value {
	if g, ok := entityNumFields[field]; ok {
		return expr.Num(g(p))
	}
	if g, ok := entityStrFields[field]; ok {
		return expr.Str(g(p))
	}
	if field == "nice" {
		if p.NiceAvail == model.Available {
			return expr.Num(float64(p.Nice))
		}
		return expr.Missing
	}
	if inode, ok := nsField(p, field); ok {
		return expr.Num(float64(inode))
	}
	return expr.Missing
}

func nsField(p *model.Process, field string) (uint64, bool) {
	m := map[string]model.NamespaceType{
		"pidns": model.NSPID, "netns": model.NSNet, "mntns": model.NSMount,
		"userns": model.NSUser, "cgroupns": model.NSCgroup,
	}
	t, ok := m[field]
	if !ok {
		return 0, false
	}
	return p.Namespaces.Get(t)
}

// RowEnv adapts an aggregated row to expr.Env for the `having` stage.
type RowEnv struct{ R *query.Row }

// Lookup resolves a field to a Value (Missing if unknown/unavailable).
func (e RowEnv) Lookup(field string) expr.Value {
	if v, ok := e.R.Metrics[model.MetricID(field)]; ok {
		if n, ok := v.Get(); ok {
			return expr.Num(n)
		}
		return expr.Missing
	}
	if v, ok := rowAggField(e.R, field); ok {
		return v
	}
	return rowLeafField(e.R, field)
}

// rowAggField resolves the aggregate/structural row fields.
func rowAggField(r *query.Row, field string) (expr.Value, bool) {
	switch field {
	case "procs":
		return expr.Num(float64(r.Procs)), true
	case "threads":
		return expr.Num(float64(r.Threads)), true
	case "children":
		return expr.Num(float64(r.Children)), true
	case "leaves":
		return expr.Num(float64(r.Leaves)), true
	case "target", "label", "comm", "name":
		return expr.Str(r.Label), true
	case "kind":
		return expr.Str(string(r.Kind)), true
	}
	return expr.Missing, false
}

// rowLeafField resolves per-leaf fields (state/wchan) available on process/thread
// rows; Missing on group rows.
func rowLeafField(r *query.Row, field string) expr.Value {
	switch field {
	case "state", "pstate":
		if s, ok := rowState(r); ok {
			return expr.Str(s)
		}
	case "wchan":
		if r.Process != nil {
			return expr.Str(r.Process.Wchan)
		}
	}
	return expr.Missing
}

// rowState returns a process/thread row's state code (empty for group rows).
func rowState(r *query.Row) (string, bool) {
	switch {
	case r.Process != nil:
		return string(r.Process.State.Code), true
	case r.Thread != nil:
		return string(r.Thread.State.Code), true
	default:
		return "", false
	}
}

// SelectPred bridges a compiled program to query.EntityPredicate.
type SelectPred struct{ Prog *expr.Program }

// EvalEntity evaluates the program against an entity.
func (s SelectPred) EvalEntity(p *model.Process) bool { return s.Prog.Eval(EntityEnv{P: p}) }

// HavingPred bridges a compiled program to query.RowPredicate.
type HavingPred struct{ Prog *expr.Program }

// EvalRow evaluates the program against an aggregated row.
func (h HavingPred) EvalRow(r *query.Row) bool { return h.Prog.Eval(RowEnv{R: r}) }

// AllowedEntityFields is the set of fields valid in a `select` expression.
func AllowedEntityFields(reg *metrics.Registry) map[string]bool {
	allowed := map[string]bool{
		"pid": true, "ppid": true, "pgid": true, "sid": true, "session": true,
		"uid": true, "gid": true, "comm": true, "name": true, "user": true, "exe": true, "app": true,
		"cgroup": true, "systemd-unit": true, "container": true, "pod": true,
		"state": true, "pstate": true, "wchan": true, "nice": true,
		"pidns": true, "netns": true, "mntns": true, "userns": true, "cgroupns": true,
	}
	addMetricFields(reg, allowed)
	return allowed
}

// AllowedRowFields is the set of fields valid in a `having` expression. Process/
// thread leaves also expose their state and (blocked) wchan.
func AllowedRowFields(reg *metrics.Registry) map[string]bool {
	allowed := map[string]bool{
		"procs": true, "threads": true, "children": true, "leaves": true,
		"target": true, "label": true, "comm": true, "name": true, "kind": true,
		"state": true, "pstate": true, "wchan": true,
	}
	addMetricFields(reg, allowed)
	return allowed
}

func addMetricFields(reg *metrics.Registry, allowed map[string]bool) {
	for _, d := range reg.All() {
		allowed[string(d.ID)] = true
		for _, al := range d.Aliases {
			allowed[al] = true
		}
	}
}
