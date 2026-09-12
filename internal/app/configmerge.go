package app

import (
	"flag"
	"os"
	"strings"

	"github.com/netikras/procfit/internal/config"
)

// applyConfigDefaults fills view flags from a discovered config file for any flag
// the user did NOT set on the command line, implementing the precedence
// built-in defaults < config < CLI (RFC §9.3). It is a no-op when --no-config is
// given or no config file is found. A malformed config is an error.
func applyConfigDefaults(fs *flag.FlagSet, qf *queryFlags) error {
	if qf.noConfig {
		return nil
	}
	d := config.Discovery{
		Explicit:  qf.configPath,
		EnvValue:  os.Getenv("PROCFIT_CONFIG"),
		ConfigDir: config.DefaultConfigDir(),
	}
	path, err := d.Discover()
	if err != nil || path == "" {
		return err
	}
	cfg, err := config.LoadFile(path, configValidator())
	if err != nil {
		return err
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	mergeViewFlags(qf, cfg, set)
	return nil
}

// mergeViewFlags copies config view fields into qf for flags the user did not
// set. Rules are data-driven so this stays within the complexity cap and adding
// a field is a one-line entry.
func mergeViewFlags(qf *queryFlags, cfg config.Config, set map[string]bool) {
	rules := []struct {
		flag  string
		apply func()
	}{
		{"group-by", func() {
			if len(cfg.GroupBy) > 0 {
				qf.groupBy = strings.Join(cfg.GroupBy, ",")
			}
		}},
		{"leaf", func() {
			if cfg.Leaf != "" {
				qf.leaf = cfg.Leaf
			}
		}},
		{"columns", func() {
			if len(cfg.Columns) > 0 {
				qf.columns = strings.Join(cfg.Columns, ",")
			}
		}},
		{"sort", func() {
			if len(cfg.Sort) > 0 {
				qf.sortSpec = joinSort(cfg.Sort)
			}
		}},
		{"metrics", func() {
			if cfg.Metrics.Profile != "" {
				qf.profile = cfg.Metrics.Profile
			}
		}},
		{"target-width", func() {
			if cfg.TargetWidth > 0 {
				qf.targetWidth = cfg.TargetWidth
			}
		}},
	}
	for _, r := range rules {
		if !set[r.flag] {
			r.apply()
		}
	}
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
