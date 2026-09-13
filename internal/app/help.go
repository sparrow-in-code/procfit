package app

import (
	"flag"
	"fmt"

	"github.com/netikras/procfit/internal/meta"
)

// setupUsage makes `--help` on a subcommand print a clear usage block: summary,
// worked examples (with a note on what to expect), and the flags (with defaults,
// courtesy of flag.PrintDefaults). Output goes to stdout. Note `-h` means
// --human (free-style); use --help for help.
func setupUsage(env Env, fs *flag.FlagSet, name, summary string, examples ...string) {
	fs.Usage = func() {
		fmt.Fprintf(env.Stdout, "%s %s — %s\n\nUsage: %s %s [flags]\n", meta.Name, name, summary, meta.Name, name)
		if len(examples) > 0 {
			fmt.Fprintf(env.Stdout, "\nExamples:\n")
			for _, e := range examples {
				fmt.Fprintf(env.Stdout, "  %s\n", e)
			}
		}
		if hasFlags(fs) {
			fmt.Fprintf(env.Stdout, "\nFlags:\n")
			old := fs.Output()
			fs.SetOutput(env.Stdout)
			fs.PrintDefaults()
			fs.SetOutput(old)
		}
	}
}

func hasFlags(fs *flag.FlagSet) bool {
	n := 0
	fs.VisitAll(func(*flag.Flag) { n++ })
	return n > 0
}

// parseExit maps a FlagSet parse error to an exit code: --help is success,
// anything else is a usage error.
func parseExit(err error) int {
	if err == flag.ErrHelp {
		return ExitOK
	}
	return ExitUsage
}
