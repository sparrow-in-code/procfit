package daemon

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/expr"
	"github.com/netikras/procfit/internal/queryspec"
)

func splitTarget(spec string) (kind, arg string, hasPrefix bool) {
	k, a, ok := strings.Cut(spec, ":")
	if !ok {
		return "", spec, false
	}
	return k, a, true
}

func errUnknownTargetKind(kind string) error {
	return fmt.Errorf("unknown target kind %q", kind)
}

func (e *Engine) instancesForPID(s string) ([]control.Instance, error) {
	pid, err := strconv.Atoi(s)
	if err != nil {
		return nil, fmt.Errorf("bad pid %q", s)
	}
	id, ok := e.ctrl.ReadIdentity(pid)
	if !ok {
		return nil, fmt.Errorf("pid %d not found", pid)
	}
	return []control.Instance{{ID: id, PID: pid}}, nil
}

func (e *Engine) instancesBySelector(sel string) ([]control.Instance, error) {
	prog, err := expr.Compile(sel)
	if err != nil {
		return nil, err
	}
	in := e.snapshotInput()
	var out []control.Instance
	for i := range in.Processes {
		p := &in.Processes[i]
		if prog.Eval(queryspec.EntityEnv{P: p}) {
			out = append(out, control.Instance{ID: p.ID, PID: p.PID})
		}
	}
	return out, nil
}

func (e *Engine) instancesByGroup(arg string) ([]control.Instance, error) {
	dim, val, ok := strings.Cut(arg, "=")
	if !ok {
		return nil, fmt.Errorf("group target must be dim=value, got %q", arg)
	}
	in := e.snapshotInput()
	var out []control.Instance
	for i := range in.Processes {
		p := &in.Processes[i]
		v := queryspec.EntityField(p, dim)
		if (v.Kind == expr.KindString && v.Str == val) ||
			(v.Kind == expr.KindNumber && strconv.FormatInt(int64(v.Num), 10) == val) {
			out = append(out, control.Instance{ID: p.ID, PID: p.PID})
		}
	}
	return out, nil
}
