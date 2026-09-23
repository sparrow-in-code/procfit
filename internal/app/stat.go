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
	"github.com/netikras/procfit/internal/queryspec"
	"github.com/netikras/procfit/internal/render"
	"github.com/netikras/procfit/internal/render/csv"
	jsonrender "github.com/netikras/procfit/internal/render/json"
	"github.com/netikras/procfit/internal/render/ndjson"
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
	setupUsage(env, fs, "stat", "repeated append-only samples",
		"procfit stat 2s                              # sample every 2s until Ctrl-C",
		"procfit stat 1s --count 3 --group-by comm --leaf none   # 3 timestamped batches",
		"procfit stat --interval 500ms --format ndjson   # machine stream (one meta record + rows)")

	// The interval may be given as a leading positional (e.g. `stat 2s --preset io`).
	// Go's flag parser stops at the first non-flag, so pull it out before parsing.
	posInterval := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		posInterval = args[0]
		args = args[1:]
	}
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
	// Host-scoped metrics render in the per-batch system matrix, not as columns.
	cols, err := render.ResolveColumnsMode(a.reg, a.processColumns(r.Columns), r.Human)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return a.streamLoop(ctx, env, r, cols, interval, *count)
}

func (a *assembly) streamLoop(ctx context.Context, env Env, r queryspec.Resolved, cols []render.Column, interval time.Duration, count int) int {
	configureSource(a.src, r)
	sampler := collect.NewSampler(a.src, a.clk)
	emit, err := a.streamEmitter(env, r, cols)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	emitted := 0
	for {
		snap, err := sampler.Sample(ctx, r.Needed)
		if err != nil {
			if ctx.Err() != nil {
				return ExitInterrupted
			}
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitRuntime
		}
		a.applyResolvers(snap.Processes)
		a.enrich(ctx, snap.Processes, r.Needed)
		res, err := a.engine.Build(query.Input{
			Generation: snap.Generation, WallTime: snap.WallTime, Elapsed: snap.Elapsed, Processes: snap.Processes,
			HostMetrics: a.collectHost(ctx, r.Needed),
		}, r.Spec)
		if err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitUsage
		}
		if err := emit(res); err != nil {
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

// streamEmitter returns a per-batch emit function for the chosen format. Table
// output prefixes each batch with a timestamp header; csv emits its header once;
// ndjson emits a metadata record then row records; each batch is append-only
// (RFC §7.3).
func (a *assembly) streamEmitter(env Env, r queryspec.Resolved, cols []render.Column) (func(*query.Result) error, error) {
	switch r.Format {
	case "ndjson":
		s := ndjson.NewStreamer(env.Stdout, a.reg)
		return s.WriteResult, nil
	case "csv":
		if err := csv.WriteHeader(env.Stdout, cols); err != nil {
			return nil, err
		}
		return func(res *query.Result) error { return csv.WriteRows(env.Stdout, res, cols) }, nil
	case "json":
		return func(res *query.Result) error { return jsonrender.Render(env.Stdout, res, a.reg) }, nil
	default: // table / wide
		return func(res *query.Result) error {
			fmt.Fprintf(env.Stdout, "== %s ==\n", res.WallTime.Format(time.RFC3339))
			// Host-scoped metrics as an aligned matrix above the table (the batch
			// timestamp already heads the section, so no separate banner).
			for _, line := range a.hostMatrix(res.HostMetrics, r.Human) {
				fmt.Fprintln(env.Stdout, line)
			}
			if len(res.HostMetrics) > 0 {
				fmt.Fprintln(env.Stdout)
			}
			return table.Render(env.Stdout, res, cols, r.TargetWidth)
		}, nil
	}
}
