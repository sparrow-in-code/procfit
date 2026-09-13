package app

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/config"
)

// Source identifies which precedence layer supplied a setting's effective value.
type Source string

const (
	SourceDefault Source = "default"
	SourceConfig  Source = "config"
	SourceEnv     Source = "env"
	SourceArgs    Source = "args"
)

// defaultPrecedence is the built-in resolution order, lowest priority first. The
// user may reorder the upper layers via --config-precedence; "default" is always
// forced to the bottom as the floor.
var defaultPrecedence = []Source{SourceDefault, SourceConfig, SourceEnv, SourceArgs}

// setting describes one configurable observation knob. The catalog below is the
// single source of truth for how each value is read from config (and, by the
// derived env var, from the environment) and printed — adding a knob is one
// entry (Open/Closed).
type setting struct {
	flag string // CLI flag name; also the print key and env-var stem
	// fromConfig extracts the value from a loaded config, reporting presence.
	// nil means the config schema carries no such value (env/args only).
	fromConfig func(config.Config) (string, bool)
}

// viewSettings is the central catalog of shared observation knobs (ps/stat/tui).
var viewSettings = []setting{
	{"group-by", func(c config.Config) (string, bool) { return strings.Join(c.GroupBy, ","), len(c.GroupBy) > 0 }},
	{"leaf", func(c config.Config) (string, bool) { return c.Leaf, c.Leaf != "" }},
	{"columns", func(c config.Config) (string, bool) { return strings.Join(c.Columns, ","), len(c.Columns) > 0 }},
	{"sort", func(c config.Config) (string, bool) { return joinSort(c.Sort), len(c.Sort) > 0 }},
	{"metrics", func(c config.Config) (string, bool) { return c.Metrics.Profile, c.Metrics.Profile != "" }},
	{"target-width", func(c config.Config) (string, bool) {
		if c.TargetWidth > 0 {
			return strconv.Itoa(c.TargetWidth), true
		}
		return "", false
	}},
	{flag: "format"},
	{flag: "select"},
	{flag: "having"},
	{flag: "number"},
	{flag: "human"},
}

// envName derives a setting's environment variable, e.g. group-by -> PROCFIT_GROUP_BY.
func envName(flagName string) string {
	return "PROCFIT_" + strings.ToUpper(strings.ReplaceAll(flagName, "-", "_"))
}

// resolveViewSettings applies each catalog setting across the precedence order,
// mutating the flagset (bound to queryFlags) so the winning layer's value takes
// effect, and returns each setting's winning source for later reporting.
func resolveViewSettings(fs *flag.FlagSet, cfg config.Config, cliSet map[string]bool, order []Source) map[string]Source {
	sources := make(map[string]Source, len(viewSettings))
	for _, s := range viewSettings {
		sources[s.flag] = resolveSetting(fs, cfg, cliSet, order, s)
	}
	return sources
}

// resolveSetting walks the layers low->high; the last layer that provides a value
// wins. Defaults and args already live in the flagset, so only a config/env win
// needs to be written back with fs.Set.
func resolveSetting(fs *flag.FlagSet, cfg config.Config, cliSet map[string]bool, order []Source, s setting) Source {
	f := fs.Lookup(s.flag)
	winner, winRaw := SourceDefault, f.DefValue
	for _, src := range order {
		if raw, ok := layerValue(src, f, cfg, cliSet, s); ok {
			winner, winRaw = src, raw
		}
	}
	if winner == SourceConfig || winner == SourceEnv {
		_ = fs.Set(s.flag, winRaw)
	}
	return winner
}

func layerValue(src Source, f *flag.Flag, cfg config.Config, cliSet map[string]bool, s setting) (string, bool) {
	switch src {
	case SourceDefault:
		return f.DefValue, true
	case SourceConfig:
		if s.fromConfig == nil {
			return "", false
		}
		return s.fromConfig(cfg)
	case SourceEnv:
		v := os.Getenv(envName(s.flag))
		return v, v != ""
	case SourceArgs:
		if cliSet[s.flag] {
			return f.Value.String(), true
		}
		return "", false
	}
	return "", false
}

// parsePrecedence turns a comma-separated spec (e.g. "config,env,args") into a
// resolution order. "default(s)" is always forced to the bottom. Unknown tokens
// are an error so a typo cannot silently change precedence.
func parsePrecedence(spec string) ([]Source, error) {
	if strings.TrimSpace(spec) == "" {
		return defaultPrecedence, nil
	}
	valid := map[string]Source{
		"default": SourceDefault, "defaults": SourceDefault,
		"config": SourceConfig, "env": SourceEnv,
		"args": SourceArgs, "cli": SourceArgs, "flags": SourceArgs,
	}
	seen := map[Source]bool{}
	order := []Source{SourceDefault}
	for _, tok := range strings.Split(spec, ",") {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "" {
			continue
		}
		src, ok := valid[tok]
		if !ok {
			return nil, fmt.Errorf("unknown precedence layer %q (want: default,config,env,args)", tok)
		}
		if src == SourceDefault || seen[src] {
			continue
		}
		seen[src] = true
		order = append(order, src)
	}
	return order, nil
}

// printEffectiveSettings prints every configurable, its resolved value, the layer
// it came from, and the env var that overrides it — for `--show-config`.
func printEffectiveSettings(env Env, fs *flag.FlagSet, sources map[string]Source) {
	fmt.Fprintf(env.Stdout, "%-14s %-28s %-8s %s\n", "SETTING", "VALUE", "SOURCE", "ENV")
	for _, s := range viewSettings {
		val := fs.Lookup(s.flag).Value.String()
		fmt.Fprintf(env.Stdout, "%-14s %-28s %-8s %s\n", s.flag, val, sources[s.flag], envName(s.flag))
	}
}
