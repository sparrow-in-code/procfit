package daemon

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/queryspec"
)

func reqToFlags(q *QueryReq) queryspec.Flags {
	return queryspec.Flags{
		GroupBy: q.GroupBy, Leaf: q.Leaf, Sort: q.Sort, Columns: q.Columns,
		Profile: q.Metrics, Select: q.Select, Having: q.Having,
	}
}

func (s *Server) control(c *ControlReq) Response {
	if c == nil {
		return Response{Version: ProtocolVersion, Error: "control: missing payload"}
	}
	res, err := s.runControl(c)
	if err != nil {
		return Response{Version: ProtocolVersion, Error: err.Error()}
	}
	if s.saver != nil {
		if err := s.saver(); err != nil {
			return Response{Version: ProtocolVersion, Error: "save state: " + err.Error()}
		}
	}
	data, _ := json.Marshal(res)
	return Response{Version: ProtocolVersion, Control: data}
}

func (s *Server) runControl(c *ControlReq) (*control.ApplyResult, error) {
	mgr := s.engine.mgr
	switch c.Op {
	case "restore":
		return mgr.Restore(targetName(c.Target), c.Force)
	case "unmanage":
		return &control.ApplyResult{Counts: map[control.FieldStatus]int{}}, mgr.Unmanage(targetName(c.Target))
	case "signal":
		instances, err := s.engine.ResolveInstances(c.Target)
		if err != nil {
			return nil, err
		}
		return mgr.Signal(instances, ports.Signal(strings.ToUpper(c.Signal))), nil
	}
	name, err := s.ensureTarget(c.Target)
	if err != nil {
		return nil, err
	}
	switch c.Op {
	case "set-nice":
		if c.Nice == nil {
			return nil, fmt.Errorf("set-nice requires a nice value")
		}
		return mgr.SetNice(name, *c.Nice, false)
	case "stop":
		return mgr.SetStop(name, true)
	case "continue":
		return mgr.SetStop(name, false)
	case "freeze":
		return mgr.SetFreeze(name, true, false, false)
	case "thaw":
		return mgr.SetFreeze(name, false, false, false)
	default:
		return nil, fmt.Errorf("unknown control op %q", c.Op)
	}
}

// ensureTarget returns an existing managed target name or creates one from the
// spec so control retains managed state (RFC §8.6).
func (s *Server) ensureTarget(spec string) (string, error) {
	name := targetName(spec)
	if s.engine.mgr.Find(name) != nil {
		return name, nil
	}
	instances, err := s.engine.ResolveInstances(spec)
	if err != nil {
		return "", err
	}
	s.engine.mgr.Manage(name, defaultMode(spec), selectorOf(spec), instances)
	return name, nil
}

func targetName(spec string) string {
	if kind, arg, ok := strings.Cut(spec, ":"); ok && kind == "managed" {
		return arg
	}
	return spec
}

func defaultMode(spec string) control.BindingMode {
	if strings.HasPrefix(spec, "pid:") || strings.HasPrefix(spec, "tid:") {
		return control.ModeSnapshot
	}
	return control.ModeFollow
}

func selectorOf(spec string) string {
	if kind, arg, ok := strings.Cut(spec, ":"); ok && kind == "selector" {
		return arg
	}
	return ""
}
