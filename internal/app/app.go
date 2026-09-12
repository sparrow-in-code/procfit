// Package app orchestrates command-line dispatch. It is a thin Facade over the
// engine, controllers, and renderers; it holds no observation or control logic
// itself (DEVELOPMENT.md §2.3). Commands register in a table so adding one does
// not touch existing command code (Open/Closed).
package app

import (
	"fmt"
	"io"
	"os"

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
		printUsage(env, cmds)
		return ExitOK
	case "-v", "--version":
		return dispatch(cmds, env, "version", nil)
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
		{name: "tui", summary: "Interactive explorer (not yet implemented)", run: cmdTUIStub},
		{name: "version", summary: "Print version", run: cmdVersion},
	}
}

func printUsage(env Env, cmds []command) {
	fmt.Fprintf(env.Stdout, "%s — %s\n\nUsage: %s [command] [flags]\n\nCommands:\n", meta.Name, meta.Description, meta.Name)
	for _, c := range cmds {
		fmt.Fprintf(env.Stdout, "  %-14s %s\n", c.name, c.summary)
	}
}

func cmdVersion(env Env, _ []string) int {
	fmt.Fprintf(env.Stdout, "%s %s (commit %s, built %s)\n", meta.Name, meta.Version, meta.Commit, meta.BuildDate)
	return ExitOK
}

func cmdTUIStub(env Env, _ []string) int {
	fmt.Fprintf(env.Stderr, "%s: the TUI is not yet implemented; use '%s ps' or '%s stat'\n", meta.Name, meta.Name, meta.Name)
	return ExitUnavailable
}
