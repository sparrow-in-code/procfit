package app

import (
	"flag"
	"fmt"

	"github.com/netikras/procfit/internal/meta"
)

// setupUsage makes `--help` on a subcommand print a clear usage block listing its
// flags (to stdout). Note `-h` means --human (free-style); use --help for help.
func setupUsage(env Env, fs *flag.FlagSet, name, summary string) {
	fs.Usage = func() {
		fmt.Fprintf(env.Stdout, "%s %s — %s\n\nUsage: %s %s [flags]\n\nFlags:\n", meta.Name, name, summary, meta.Name, name)
		old := fs.Output()
		fs.SetOutput(env.Stdout)
		fs.PrintDefaults()
		fs.SetOutput(old)
	}
}

// parseExit maps a FlagSet parse error to an exit code: --help is success,
// anything else is a usage error.
func parseExit(err error) int {
	if err == flag.ErrHelp {
		return ExitOK
	}
	return ExitUsage
}
