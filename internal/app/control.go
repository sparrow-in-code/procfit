package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/collect"
	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/expr"
	"github.com/netikras/procfit/internal/meta"
	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
	"github.com/netikras/procfit/internal/procfs"
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/state"
	"github.com/netikras/procfit/internal/sysclock"
)

// ctlAsm wires the control plane: observation source (for resolution), the
// controller port, the manager, and the persistent state store.
type ctlAsm struct {
	src   ports.ProcessSource
	ctrl  ports.Controller
	clk   ports.Clock
	mgr   *control.Manager
	store *state.Store
}

// newControlAsmFn is the constructor commands use; overridable in tests.
var newControlAsmFn = newControlAsm

func newControlAsm(stateDir string) (*ctlAsm, error) {
	src, err := procfs.New()
	if err != nil {
		return nil, err
	}
	ctrl := procfs.NewController(src)
	clk := sysclock.New()
	bootID, _ := src.BootID()
	store, err := state.Open(stateDir)
	if err != nil {
		return nil, err
	}
	mgr := control.NewManager(ctrl, clk, control.NewSafeguards(os.Getpid(), os.Getppid()), bootID)
	if st, ok, err := store.Load(); err == nil && ok {
		mgr.LoadState(st)
	}
	return &ctlAsm{src: src, ctrl: ctrl, clk: clk, mgr: mgr, store: store}, nil
}

func (c *ctlAsm) save() error { return c.store.Save(c.mgr.State()) }

// resolveInstances resolves a target spec against a live instant snapshot into
// concrete instances (RFC §8.5). Supports pid:, managed:, selector:, group:, and
// bare numeric PIDs.
func (c *ctlAsm) resolveInstances(spec string) ([]control.Instance, error) {
	kind, arg, _ := strings.Cut(spec, ":")
	if arg == "" { // bare numeric pid
		if n, err := strconv.Atoi(spec); err == nil {
			return c.instancesForPID(n)
		}
		return nil, fmt.Errorf("ambiguous target %q; use a pid:, managed:, selector: or group: prefix", spec)
	}
	switch kind {
	case "pid":
		n, err := strconv.Atoi(arg)
		if err != nil {
			return nil, fmt.Errorf("bad pid %q", arg)
		}
		return c.instancesForPID(n)
	case "managed":
		t := c.mgr.Find(arg)
		if t == nil {
			return nil, fmt.Errorf("managed target %q not found", arg)
		}
		out := make([]control.Instance, 0, len(t.Bindings))
		for _, b := range t.Bindings {
			out = append(out, control.Instance{ID: b.ID, PID: b.PID})
		}
		return out, nil
	case "selector":
		return c.instancesBySelector(arg)
	case "group":
		return c.instancesByGroup(arg)
	default:
		return nil, fmt.Errorf("unknown target kind %q", kind)
	}
}

func (c *ctlAsm) instancesForPID(pid int) ([]control.Instance, error) {
	id, ok := c.ctrl.ReadIdentity(pid)
	if !ok {
		return nil, fmt.Errorf("pid %d not found", pid)
	}
	return []control.Instance{{ID: id, PID: pid}}, nil
}

func (c *ctlAsm) snapshot() ([]model.Process, error) {
	s := collect.NewSampler(c.src, c.clk)
	snap, err := s.Sample(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	return snap.Processes, nil
}

func (c *ctlAsm) instancesBySelector(sel string) ([]control.Instance, error) {
	prog, err := expr.Compile(sel)
	if err != nil {
		return nil, err
	}
	procs, err := c.snapshot()
	if err != nil {
		return nil, err
	}
	var out []control.Instance
	for i := range procs {
		p := &procs[i]
		if prog.Eval(queryspec.EntityEnv{P: p}) {
			out = append(out, control.Instance{ID: p.ID, PID: p.PID})
		}
	}
	return out, nil
}

func (c *ctlAsm) instancesByGroup(arg string) ([]control.Instance, error) {
	dim, val, ok := strings.Cut(arg, "=")
	if !ok {
		return nil, fmt.Errorf("group target must be dim=value, got %q", arg)
	}
	procs, err := c.snapshot()
	if err != nil {
		return nil, err
	}
	var out []control.Instance
	for i := range procs {
		p := &procs[i]
		if v := queryspec.EntityField(p, dim); v.Kind == expr.KindString && v.Str == val {
			out = append(out, control.Instance{ID: p.ID, PID: p.PID})
		} else if v.Kind == expr.KindNumber && fmt.Sprintf("%d", int64(v.Num)) == val {
			out = append(out, control.Instance{ID: p.ID, PID: p.PID})
		}
	}
	return out, nil
}

// resolveStateDir returns the runtime state directory (RFC §16.1). Prefers
// $XDG_RUNTIME_DIR/procfit; falls back to a private temp dir with a warning.
func resolveStateDir(override string) (string, bool) {
	if override != "" {
		return override, false
	}
	if x := os.Getenv("XDG_RUNTIME_DIR"); x != "" {
		return filepath.Join(x, meta.Name), false
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", meta.Name, os.Getuid())), true
}

// exitForResult maps aggregate control outcomes to an exit code (RFC §8.7).
func exitForResult(res *control.ApplyResult) int {
	if res == nil || len(res.Results) == 0 {
		return ExitOK
	}
	good := res.Counts[control.StatusApplied] + res.Counts[control.StatusRestored] + res.Counts[control.StatusUnchanged]
	if res.Counts[control.StatusDrifted] > 0 || res.Counts[control.StatusStale] > 0 {
		if good == 0 {
			return ExitConflict
		}
		return ExitPartial
	}
	if res.Counts[control.StatusDenied] > 0 {
		if good == 0 {
			return ExitPermission
		}
		return ExitPartial
	}
	if res.Counts[control.StatusFailed] > 0 || res.Counts[control.StatusVanished] > 0 {
		return ExitPartial
	}
	return ExitOK
}

func printResult(env Env, res *control.ApplyResult) {
	for _, r := range res.Results {
		if r.Error != "" {
			fmt.Fprintf(env.Stdout, "  pid %d: %s (%s)\n", r.PID, r.Status, r.Error)
		} else {
			fmt.Fprintf(env.Stdout, "  pid %d: %s\n", r.PID, r.Status)
		}
	}
}
