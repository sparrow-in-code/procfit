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
	setupUsage(env, fs, "ps", "one-shot snapshot/table")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if err := applyConfigDefaults(fs, qf); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
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
