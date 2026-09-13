package app

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/netikras/procfit/internal/collect"
	"github.com/netikras/procfit/internal/control"
	"github.com/netikras/procfit/internal/meta"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/render"
	"github.com/netikras/procfit/internal/tui"
)

// cmdTUI runs the interactive explorer (RFC §7.1, §19). It requires a TTY.
func cmdTUI(env Env, args []string) int {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	qf := bindQueryFlags(fs)
	setupUsage(env, fs, "tui", "interactive explorer (default on a TTY)",
		"procfit tui                                  # nav g(group) t(leaf) s/S / u; control n(ice) x/c z/Z R (confirm y)",
		"procfit tui --group-by name --sort cpu:desc  # start grouped by name, sorted by CPU",
		"procfit tui -h                               # start with human units (toggle live with 'u')")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	if qf.showConfig {
		printEffectiveSettings(env, fs, qf.sources)
		return ExitOK
	}
	if !env.IsTTY {
		fmt.Fprintf(env.Stderr, "%s: the TUI needs an interactive terminal; use '%s ps' or '%s stat'\n", meta.Name, meta.Name, meta.Name)
		return ExitUsage
	}
	a, err := newAssemblyFn()
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	cli, err := tui.RunTerminal(a.tuiRefresh(context.Background()), tuiControl(), qf.toSpecFlags())
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	if cli != "" {
		fmt.Fprintln(env.Stdout, cli) // reproducible CLI for the final view (§27 Phase 3 exit)
	}
	return ExitOK
}

// tuiRefresh builds a RefreshFunc backed by a persistent sampler so rates warm up
// across ticks; each refresh resolves the flags, samples, enriches, and builds.
func (a *assembly) tuiRefresh(ctx context.Context) tui.RefreshFunc {
	sampler := collect.NewSampler(a.src, a.clk)
	return func(f queryspec.Flags) (*query.Result, []render.Column, error) {
		r, err := queryspec.Build(a.reg, a.dims, f)
		if err != nil {
			return nil, nil, err
		}
		setThreadEnum(a.src, r.Spec.Leaf == query.LeafThread)
		snap, err := sampler.Sample(ctx, r.Needed)
		if err != nil {
			return nil, nil, err
		}
		a.applyResolvers(snap.Processes)
		a.enrich(ctx, snap.Processes, r.Needed)
		res, err := a.engine.Build(query.Input{
			Generation: snap.Generation, WallTime: snap.WallTime, Elapsed: snap.Elapsed, Processes: snap.Processes,
		}, r.Spec)
		if err != nil {
			return nil, nil, err
		}
		cols, err := render.ResolveColumnsMode(a.reg, r.Columns, r.Human)
		if err != nil {
			return nil, nil, err
		}
		return res, cols, nil
	}
}

// tuiControl builds the TUI's ControlFunc: it applies (or, DryRun, previews) a
// single-process control action via the same managed-target path as the CLI
// (auto-managing the pid), returning a summary string instead of printing — so
// it never corrupts the screen. State is loaded/saved per action.
func tuiControl() tui.ControlFunc {
	return func(req tui.ControlRequest) (string, error) {
		dir, _ := resolveStateDir("")
		c, err := newControlAsmFn(dir)
		if err != nil {
			return "", err
		}
		insts := c.instancesForPIDs(req.PIDs)
		if len(insts) == 0 {
			return "", fmt.Errorf("no live instances for the selection")
		}
		name := tuiTargetName(req)
		c.mgr.Manage(name, control.ModeSnapshot, "", insts)
		summary, err := applyTUIControl(c, name, req)
		if err != nil {
			return "", err
		}
		if !req.DryRun {
			if err := c.save(); err != nil {
				return "", err
			}
		}
		return summary, nil
	}
}

// tuiTargetName names the managed target for a TUI action: pid:N for a single
// process (so the CLI's `set`/`restore pid:N` share it), or tui:<label> for a
// group (stable across repeated actions on the same group, enabling restore).
func tuiTargetName(req tui.ControlRequest) string {
	if len(req.PIDs) == 1 {
		return targetName(fmt.Sprintf("pid:%d", req.PIDs[0]))
	}
	return "tui:" + req.Label
}

// instancesForPIDs resolves each pid to a live instance, skipping any that
// vanished; the caller treats an empty result as "nothing to do".
func (c *ctlAsm) instancesForPIDs(pids []int) []control.Instance {
	var out []control.Instance
	for _, pid := range pids {
		insts, err := c.instancesForPID(pid)
		if err != nil {
			continue
		}
		out = append(out, insts...)
	}
	return out
}

// applyTUIControl performs one control action. Nice has a real dry-run through
// the manager (surfacing safeguards); the other actions have no dry-run at the
// manager level, so their preview is synthesized and only the confirmed apply
// touches the process.
func applyTUIControl(c *ctlAsm, name string, req tui.ControlRequest) (string, error) {
	if req.Kind == tui.CtrlNice {
		res, err := c.mgr.SetNice(name, req.Nice, req.DryRun)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("nice→%d  %s", req.Nice, summarizeResult(res)), nil
	}
	if req.DryRun {
		return fmt.Sprintf("would %s %d process(es) [%s]", req.Kind.Verb(), len(req.PIDs), req.Label), nil
	}
	var res *control.ApplyResult
	switch req.Kind {
	case tui.CtrlStop:
		res, _ = c.mgr.SetStop(name, true)
	case tui.CtrlContinue:
		res, _ = c.mgr.SetStop(name, false)
	case tui.CtrlFreeze:
		res, _ = c.mgr.SetFreeze(name, true)
	case tui.CtrlThaw:
		res, _ = c.mgr.SetFreeze(name, false)
	case tui.CtrlRestore:
		r, err := c.mgr.Restore(name, false)
		if err != nil {
			return "", err
		}
		res = r
	}
	return summarizeResult(res), nil
}

// summarizeResult renders an ApplyResult as a compact one-line status: per-pid
// for a single process, or aggregated status counts for a group.
func summarizeResult(res *control.ApplyResult) string {
	if res == nil || len(res.Results) == 0 {
		return "no matching instances"
	}
	if len(res.Results) == 1 {
		r := res.Results[0]
		if r.Error != "" {
			return fmt.Sprintf("pid %d: %s (%s)", r.PID, r.Status, r.Error)
		}
		return fmt.Sprintf("pid %d: %s", r.PID, r.Status)
	}
	parts := make([]string, 0, len(res.Counts))
	for status, n := range res.Counts {
		parts = append(parts, fmt.Sprintf("%s=%d", status, n))
	}
	sort.Strings(parts)
	return fmt.Sprintf("%d procs: %s", len(res.Results), strings.Join(parts, " "))
}
