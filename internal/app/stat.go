package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/netikras/procfit/internal/collect"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/render"
	"github.com/netikras/procfit/internal/render/table"
)

// cmdStat implements `procfit stat [interval]`: repeated append-only samples
// (RFC §7.3).
func cmdStat(env Env, args []string) int {
	fs := flag.NewFlagSet("stat", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	qf := bindQueryFlags(fs)
	intervalFlag := fs.Duration("interval", time.Second, "sample interval")
	count := fs.Int("count", 0, "number of sample batches to emit (0 = until interrupted)")

	// The interval may be given as a leading positional (e.g. `stat 2s --preset io`).
	// Go's flag parser stops at the first non-flag, so pull it out before parsing.
	posInterval := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		posInterval = args[0]
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	interval := *intervalFlag
	if posInterval != "" {
		d, err := time.ParseDuration(posInterval)
		if err != nil {
			fmt.Fprintf(env.Stderr, "invalid interval %q\n", posInterval)
			return ExitUsage
		}
		interval = d
	}

	a, err := newAssemblyFn()
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	r, err := a.resolveQuery(qf)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	cols, err := render.ResolveColumns(a.reg, r.columns)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return a.streamLoop(ctx, env, r, cols, interval, *count)
}

func (a *assembly) streamLoop(ctx context.Context, env Env, r resolved, cols []render.Column, interval time.Duration, count int) int {
	sampler := collect.NewSampler(a.src, a.clk)
	emitted := 0
	for {
		snap, err := sampler.Sample(ctx, r.needed)
		if err != nil {
			if ctx.Err() != nil {
				return ExitInterrupted
			}
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitRuntime
		}
		res, err := a.engine.Build(query.Input{
			Generation: snap.Generation, WallTime: snap.WallTime, Elapsed: snap.Elapsed, Processes: snap.Processes,
		}, r.spec)
		if err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitUsage
		}
		fmt.Fprintf(env.Stdout, "== %s ==\n", snap.WallTime.Format(time.RFC3339))
		if err := table.Render(env.Stdout, res, cols); err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitRuntime
		}
		emitted++
		if count > 0 && emitted >= count {
			return ExitOK
		}
		select {
		case <-ctx.Done():
			return ExitInterrupted
		case <-a.wait(interval):
		}
	}
}
