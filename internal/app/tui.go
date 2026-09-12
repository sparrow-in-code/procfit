package app

import (
	"context"
	"flag"
	"fmt"

	"github.com/netikras/procfit/internal/collect"
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
	if err := fs.Parse(args); err != nil {
		return ExitUsage
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
	cli, err := tui.RunTerminal(a.tuiRefresh(context.Background()), qf.toSpecFlags())
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
		cols, err := render.ResolveColumns(a.reg, r.Columns)
		if err != nil {
			return nil, nil, err
		}
		return res, cols, nil
	}
}
