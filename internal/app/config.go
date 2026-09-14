package app

import (
	"flag"
	"fmt"
	"os"

	"github.com/netikras/procfit/internal/config"
	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
)

// configValidator builds a config.Validator from the registries, without needing
// /proc access (so `config` works anywhere).
func configValidator() config.Validator {
	reg := metrics.NewDefault()
	dims := query.NewDimensions()
	return config.Validator{
		HasMetric:    func(s string) bool { return reg.Has(s) },
		HasDimension: func(s string) bool { return dims.Has(s) },
		IsRateField: func(s string) bool {
			d, ok := reg.Get(s)
			return ok && d.IsRate()
		},
		EntityFields: queryspec.AllowedEntityFields(reg),
		KnownProfile: metrics.IsKnownProfile,
	}
}

// cmdConfig implements `procfit config check|convert|dump` (RFC §9.7).
func cmdConfig(env Env, args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(env.Stdout, configHelp)
		if len(args) == 0 {
			return ExitUsage
		}
		return ExitOK
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "check":
		return configCheck(env, rest)
	case "convert":
		return configConvert(env, rest)
	case "dump":
		return configDump(env, rest)
	default:
		fmt.Fprintf(env.Stderr, "unknown config subcommand %q\n", sub)
		fmt.Fprint(env.Stderr, configHelp)
		return ExitUsage
	}
}

const configHelp = `Usage: procfit config check|convert|dump ...

Examples:
  procfit config check                       # validate the discovered config (or defaults)
  procfit config check ./config.yaml         # strictly validate a specific file
  procfit config convert config.toml --to yaml   # convert between formats
  procfit config dump --effective            # print normalized config + defaults
`

func configCheck(env Env, args []string) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(env.Stdout, "Usage: procfit config check [file]\n\nValidate a config file (or the discovered one). Exit 2 on error.")
		return ExitOK
	}
	path, err := discoverConfigPath(args)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	if path == "" {
		fmt.Fprintln(env.Stdout, "no config file found; built-in defaults are valid")
		return ExitOK
	}
	if _, err := config.LoadFile(path, configValidator()); err != nil {
		fmt.Fprintf(env.Stderr, "%s: %v\n", path, err)
		return ExitUsage
	}
	fmt.Fprintf(env.Stdout, "%s: OK\n", path)
	return ExitOK
}

func discoverConfigPath(args []string) (string, error) {
	explicit := ""
	if len(args) > 0 {
		explicit = args[0]
	}
	d := config.Discovery{Explicit: explicit, ConfigDir: config.DefaultConfigDir()}
	return d.Discover()
}

func configConvert(env Env, args []string) int {
	fs := flag.NewFlagSet("config convert", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	to := fs.String("to", "", "target format: toml|yaml|json")
	setupUsage(env, fs, "config convert", "convert a config between formats via the canonical model",
		"procfit config convert config.toml --to yaml   # prints YAML equivalent to stdout")
	file, rest := extractPositional(args)
	if err := fs.Parse(rest); err != nil {
		return parseExit(err)
	}
	if file == "" {
		fs.Usage()
		return ExitUsage
	}
	format, ok := config.FormatFromExt("." + *to)
	if !ok {
		fmt.Fprintf(env.Stderr, "invalid target format %q\n", *to)
		return ExitUsage
	}
	c, err := config.LoadFile(file, configValidator())
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	data, err := config.Encode(c, format)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	env.Stdout.Write(data)
	return ExitOK
}

func configDump(env Env, args []string) int {
	fs := flag.NewFlagSet("config dump", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	effective := fs.Bool("effective", false, "print the fully resolved configuration")
	format := fs.String("format", "toml", "output format: toml|yaml|json")
	cfgPath := fs.String("config", "", "config file to load")
	setupUsage(env, fs, "config dump", "print the effective (normalized + defaulted) config",
		"procfit config dump --effective                 # discovered config + built-in defaults",
		"procfit config dump --config ./c.toml --format json   # a specific file as JSON")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	_ = *effective // dump always prints the effective/normalized config for MVP

	outFmt, ok := config.FormatFromExt("." + *format)
	if !ok {
		fmt.Fprintf(env.Stderr, "invalid format %q\n", *format)
		return ExitUsage
	}

	c, err := loadEffectiveConfig(*cfgPath)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitUsage
	}
	data, err := config.Encode(c, outFmt)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	env.Stdout.Write(data)
	return ExitOK
}

// extractPositional pulls the first non-flag argument out of args (Go's flag
// parser stops at the first positional, so a file given before flags would hide
// them). Returns the positional and the remaining args with it removed.
func extractPositional(args []string) (string, []string) {
	for i, a := range args {
		if len(a) == 0 || a[0] != '-' {
			rest := append([]string{}, args[:i]...)
			rest = append(rest, args[i+1:]...)
			return a, rest
		}
	}
	return "", args
}

func loadEffectiveConfig(explicit string) (config.Config, error) {
	v := configValidator()
	d := config.Discovery{Explicit: explicit, EnvValue: os.Getenv("PROCFIT_CONFIG"), ConfigDir: config.DefaultConfigDir()}
	path, err := d.Discover()
	if err != nil {
		return config.Config{}, err
	}
	if path == "" {
		// No file: return validated built-in defaults.
		return config.Load([]byte("version = 1\n"), config.FormatTOML, v)
	}
	return config.LoadFile(path, v)
}
