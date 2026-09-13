package app

import (
	"context"
	"flag"
	"fmt"
)

// cmdPS implements `procfit ps`: a one-shot snapshot/table (RFC §7.2).
func cmdPS(env Env, args []string) int {
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	qf := bindQueryFlags(fs)
	instant := fs.Bool("instant", false, "skip the warm-up second sample; rate metrics render unavailable")
	setupUsage(env, fs, "ps", "one-shot snapshot/table",
		"procfit ps                                  # flat process list, raw bytes",
		"procfit ps --group-by comm --leaf none --sort cpu:desc -n 10   # top 10 comms by CPU",
		"procfit ps --group-by name --leaf process -h   # fuller names, human units (K/M/G)",
		"procfit ps --select 'uid == 0' --having 'rss > 100M' --format json   # root procs over 100MiB as JSON")
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

	in, err := a.sampleForResult(context.Background(), r, *instant)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	res, err := a.engine.Build(in, r.Spec)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	return a.renderResult(env, res, r.Format, r.Columns, r.Human, r.TargetWidth)
}
