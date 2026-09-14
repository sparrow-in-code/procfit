package app

import (
	"flag"
	"fmt"
	"time"

	"github.com/netikras/procfit/internal/history"
)

// cmdHistory prints recent control-history events from the opt-in audit log
// (RFC §16.1), the same log surfaced in the TUI detail overlay.
func cmdHistory(env Env, args []string) int {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	pid := fs.Int("pid", 0, "only events for this pid (0 = all)")
	limit := fs.Int("n", 50, "max events, most recent")
	stateDir := fs.String("state-dir", "", "runtime state directory")
	setupUsage(env, fs, "history", "print recent control-history events (opt-in audit log)",
		"procfit history                    # recent control actions",
		"procfit history --pid 1234 -n 20   # last 20 events for a pid")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	dir, _ := resolveStateDir(*stateDir)
	keep := func(e history.Event) bool { return *pid == 0 || e.PID == *pid }
	evs, err := history.ReadRecent(historyDir(dir), keep, *limit)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	if len(evs) == 0 {
		fmt.Fprintln(env.Stdout, "no control history (enable [state].history to record)")
		return ExitOK
	}
	for _, e := range evs {
		line := fmt.Sprintf("%s  %-9s pid %d", e.Time.Format(time.RFC3339), e.Action, e.PID)
		if e.Field != "" {
			line += " " + e.Field
		}
		if e.From != "" || e.To != "" {
			line += fmt.Sprintf(" %s→%s", e.From, e.To)
		}
		if e.Target != "" {
			line += "  [" + e.Target + "]"
		}
		fmt.Fprintln(env.Stdout, line)
	}
	return ExitOK
}
