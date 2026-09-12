package app

import (
	"github.com/netikras/procfit/internal/expr"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/query"
)

// entityEnv adapts a process to expr.Env for the `select` stage. Metric values
// come from the computed metric map; structural fields from the entity.
type entityEnv struct{ p *model.Process }

func (e entityEnv) Lookup(field string) expr.Value {
	if v, ok := e.p.Metrics[model.MetricID(field)]; ok {
		if n, ok := v.Get(); ok {
			return expr.Num(n)
		}
		return expr.Missing
	}
	return entityField(e.p, field)
}

func entityField(p *model.Process, field string) expr.Value {
	switch field {
	case "pid":
		return expr.Num(float64(p.PID))
	case "ppid":
		return expr.Num(float64(p.PPID))
	case "pgid":
		return expr.Num(float64(p.PGID))
	case "sid", "session":
		return expr.Num(float64(p.SID))
	case "uid":
		return expr.Num(float64(p.UID))
	case "gid":
		return expr.Num(float64(p.GID))
	case "comm":
		return expr.Str(p.Comm)
	case "exe":
		return expr.Str(p.Exe)
	case "app":
		return expr.Str(p.AppID)
	case "cgroup":
		return expr.Str(p.CgroupPath)
	case "systemd-unit":
		return expr.Str(p.SystemdUnit)
	case "container":
		return expr.Str(p.ContainerID)
	case "pod":
		return expr.Str(p.PodUID)
	case "state":
		return expr.Str(string(p.State.Code))
	case "nice":
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

// rowEnv adapts an aggregated row to expr.Env for the `having` stage.
type rowEnv struct{ r *query.Row }

func (e rowEnv) Lookup(field string) expr.Value {
	if v, ok := e.r.Metrics[model.MetricID(field)]; ok {
		if n, ok := v.Get(); ok {
			return expr.Num(n)
		}
		return expr.Missing
	}
	switch field {
	case "procs":
		return expr.Num(float64(e.r.Procs))
	case "threads":
		return expr.Num(float64(e.r.Threads))
	case "children":
		return expr.Num(float64(e.r.Children))
	case "leaves":
		return expr.Num(float64(e.r.Leaves))
	case "target", "label", "comm":
		return expr.Str(e.r.Label)
	case "kind":
		return expr.Str(string(e.r.Kind))
	}
	return expr.Missing
}

// selectPred / havingPred bridge a compiled program to the query predicates.
type selectPred struct{ prog *expr.Program }

func (s selectPred) EvalEntity(p *model.Process) bool { return s.prog.Eval(entityEnv{p}) }

type havingPred struct{ prog *expr.Program }

func (h havingPred) EvalRow(r *query.Row) bool { return h.prog.Eval(rowEnv{r}) }

// entityAllowedFields is the set of fields valid in a `select` expression.
func (a *assembly) entityAllowedFields() map[string]bool {
	allowed := map[string]bool{
		"pid": true, "ppid": true, "pgid": true, "sid": true, "session": true,
		"uid": true, "gid": true, "comm": true, "exe": true, "app": true,
		"cgroup": true, "systemd-unit": true, "container": true, "pod": true,
		"state": true, "nice": true, "pidns": true, "netns": true, "mntns": true,
		"userns": true, "cgroupns": true,
	}
	for _, d := range a.reg.All() {
		allowed[string(d.ID)] = true
		for _, al := range d.Aliases {
			allowed[al] = true
		}
	}
	return allowed
}

// rowAllowedFields is the set of fields valid in a `having` expression.
func (a *assembly) rowAllowedFields() map[string]bool {
	allowed := map[string]bool{
		"procs": true, "threads": true, "children": true, "leaves": true,
		"target": true, "label": true, "comm": true, "kind": true,
	}
	for _, d := range a.reg.All() {
		allowed[string(d.ID)] = true
		for _, al := range d.Aliases {
			allowed[al] = true
		}
	}
	return allowed
}
