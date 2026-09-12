// Package app orchestrates command-line dispatch. It is a thin Facade over the
// engine, controllers, and renderers; it holds no observation or control logic
// itself (DEVELOPMENT.md §2.3). Commands register in a table so adding one does
// not touch existing command code (Open/Closed).
package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/netikras/procfit/internal/meta"
)

// Exit codes (RFC §8.7).
const (
	ExitOK          = 0
	ExitRuntime     = 1
	ExitUsage       = 2
	ExitUnavailable = 3
	ExitPermission  = 4
	ExitNotFound    = 5
	ExitPartial     = 6
	ExitConflict    = 7
	ExitDaemon      = 10
	ExitInterrupted = 130
)

// Env carries process I/O so commands are testable without touching os.*.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	// IsTTY reports whether Stdout is an interactive terminal.
	IsTTY bool
}

// command is one CLI subcommand.
type command struct {
	name    string
	summary string
	run     func(env Env, args []string) int
}

// Run is the entry point. It returns a process exit code.
func Run(args []string) int {
	env := Env{Stdout: os.Stdout, Stderr: os.Stderr, IsTTY: isTerminal(os.Stdout)}
	return run(env, args)
}

func run(env Env, args []string) int {
	cmds := commands()
	if len(args) == 0 {
		// Default command depends on whether stdout is a TTY (RFC §7.1).
		if env.IsTTY {
			return dispatch(cmds, env, "tui", nil)
		}
		fmt.Fprintf(env.Stderr, "%s: no TTY on stdout; use '%s ps' or '%s stat' for non-interactive output\n", meta.Name, meta.Name, meta.Name)
		return ExitUsage
	}
	name := args[0]
	rest := args[1:]
	switch name {
	case "-h", "--help", "help":
		if name == "help" && len(rest) >= 1 {
			return dispatch(cmds, env, rest[0], []string{"--help"})
		}
		printUsage(env, cmds)
		return ExitOK
	case "-v", "--version":
		return dispatch(cmds, env, "version", nil)
	}
	// Leading flags with no explicit command belong to the default command
	// (RFC §7.1: TUI on a TTY), e.g. `procfit --leaf process`.
	if strings.HasPrefix(name, "-") {
		if env.IsTTY {
			return dispatch(cmds, env, "tui", args)
		}
		fmt.Fprintf(env.Stderr, "%s: flags need a command; try '%s ps %s' (see '%s help')\n",
			meta.Name, meta.Name, strings.Join(args, " "), meta.Name)
		return ExitUsage
	}
	return dispatch(cmds, env, name, rest)
}

func dispatch(cmds []command, env Env, name string, args []string) int {
	for _, c := range cmds {
		if c.name == name {
			return c.run(env, args)
		}
	}
	fmt.Fprintf(env.Stderr, "%s: unknown command %q (try '%s help')\n", meta.Name, name, meta.Name)
	return ExitUsage
}

func commands() []command {
	return []command{
		{name: "ps", summary: "One-shot snapshot/table", run: cmdPS},
		{name: "stat", summary: "Repeated append-only samples", run: cmdStat},
		{name: "metrics", summary: "List metrics (metrics list)", run: cmdMetrics},
		{name: "capabilities", summary: "Explain available/missing collectors", run: cmdCapabilities},
		{name: "config", summary: "Validate/convert/dump configuration", run: cmdConfig},
		{name: "managed", summary: "List managed targets and bindings", run: cmdManaged},
		{name: "manage", summary: "Add managed membership, optionally control", run: cmdManage},
		{name: "set", summary: "Apply/update control on a target", run: cmdSet},
		{name: "restore", summary: "Restore captured original control fields", run: cmdRestore},
		{name: "unmanage", summary: "Remove managed membership", run: cmdUnmanage},
		{name: "signal", summary: "Send a signal to a resolved target", run: cmdSignal},
		{name: "daemon", summary: "Run/query the per-user engine daemon", run: cmdDaemon},
		{name: "service", summary: "Install/remove the user systemd unit", run: cmdService},
		{name: "tui", summary: "Interactive explorer (default on a TTY)", run: cmdTUI},
		{name: "version", summary: "Print version", run: cmdVersion},
	}
}

func printUsage(env Env, cmds []command) {
	fmt.Fprintf(env.Stdout, "%s — %s\n\nUsage: %s [command] [flags]\n\nCommands:\n", meta.Name, meta.Description, meta.Name)
	for _, c := range cmds {
		fmt.Fprintf(env.Stdout, "  %-14s %s\n", c.name, c.summary)
	}
	fmt.Fprint(env.Stdout, commonFlagsHelp)
	fmt.Fprintf(env.Stdout, "\nRun '%s <command> --help' for a command's full flags.\n", meta.Name)
}

// commonFlagsHelp summarizes the shared view flags (ps/stat/tui) so they are
// discoverable from the top-level help.
const commonFlagsHelp = `
Common view flags (ps, stat, tui):
  --group-by DIM[,DIM]   group by dimensions (comm, name, user, app, pidns, ...) or 'none'
  --leaf process|thread|none   terminal rows under groups (process = expand groups)
  --sort FIELD[:asc|desc][,...]   multi-key sort (e.g. cpu:desc)
  --columns COL[,COL]    explicit columns
  --metrics PROFILE      metric profile (light|io|process|all|...)
  --metric [+|-]NAME     add/remove a metric (repeatable)
  --select EXPR          filter entities (e.g. 'uid == 0')
  --having EXPR          filter aggregated rows (e.g. 'cpu > 5')
  -n, --number N         show only the top N rows after sorting
  -h, --human            human-readable units (K/M/G); default is raw bytes
  --target-width N       cap the TARGET column width (0 = auto)
  --format FMT           table|wide|json|ndjson|csv
  --config PATH          load a config file (--no-config to ignore)
`

func cmdVersion(env Env, _ []string) int {
	fmt.Fprintf(env.Stdout, "%s %s (commit %s, built %s)\n", meta.Name, meta.Version, meta.Commit, meta.BuildDate)
	return ExitOK
}
