package app

import (
	"context"
	"flag"
	"fmt"
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
		spec := fmt.Sprintf("pid:%d", req.PID)
		name := targetName(spec)
		if c.mgr.Find(name) == nil {
			insts, err := c.resolveInstances(spec)
			if err != nil {
				return "", err
			}
			c.mgr.Manage(name, defaultMode(spec), selectorOf(spec), insts)
		}
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
		return fmt.Sprintf("would %s pid %d (%s)", req.Kind.Verb(), req.PID, req.Label), nil
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

// summarizeResult renders an ApplyResult as a compact one-line status.
func summarizeResult(res *control.ApplyResult) string {
	if res == nil || len(res.Results) == 0 {
		return "no matching instances"
	}
	parts := make([]string, 0, len(res.Results))
	for _, r := range res.Results {
		if r.Error != "" {
			parts = append(parts, fmt.Sprintf("pid %d: %s (%s)", r.PID, r.Status, r.Error))
		} else {
			parts = append(parts, fmt.Sprintf("pid %d: %s", r.PID, r.Status))
		}
	}
	return strings.Join(parts, "; ")
}
