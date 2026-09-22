package app

import (
	"flag"
	"os"
	"strings"

	"github.com/netikras/procfit/internal/config"
)

// applyConfigDefaults resolves the shared view flags across the layered
// precedence (default < config < env < args by default, reorderable via
// --config-precedence; RFC §9.3). The config file is discovered and loaded
// unless --no-config is given; env and args still apply either way. The winning
// source per setting is recorded on qf for --show-config. A malformed config is
// an error.
func applyConfigDefaults(fs *flag.FlagSet, qf *queryFlags) error {
	order, err := parsePrecedence(qf.configPrec)
	if err != nil {
		return err
	}
	cfg := config.Config{}
	if !qf.noConfig {
		cfg, err = discoverAndLoad(qf)
		if err != nil {
			return err
		}
	}
	cliSet := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { cliSet[f.Name] = true })
	// Fold aliases into their canonical flag so args detection is accurate.
	if cliSet["n"] {
		cliSet["number"] = true
	}
	if cliSet["h"] {
		cliSet["human"] = true
	}
	if cliSet["metrics"] { // deprecated alias for --profile
		cliSet["profile"] = true
	}
	qf.sources = resolveViewSettings(fs, cfg, cliSet, order)
	return nil
}

// discoverAndLoad finds and strictly loads the config file, or returns a zero
// Config when none is found (so env/args resolution still runs).
func discoverAndLoad(qf *queryFlags) (config.Config, error) {
	d := config.Discovery{
		Explicit:  qf.configPath,
		EnvValue:  os.Getenv("PROCFIT_CONFIG"),
		ConfigDir: config.DefaultConfigDir(),
	}
	path, err := d.Discover()
	if err != nil || path == "" {
		return config.Config{}, err
	}
	return config.LoadFile(path, configValidator())
}

func joinSort(keys []config.SortKey) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		dir := k.Direction
		if dir == "" {
			dir = "asc"
		}
		parts = append(parts, k.Field+":"+dir)
	}
	return strings.Join(parts, ",")
}
