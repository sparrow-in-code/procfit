package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/netikras/procfit/internal/metrics"
)

// cmdMetrics implements `procfit metrics list` (RFC §8, §13).
func cmdMetrics(env Env, args []string) int {
	sub := "list"
	rest := args
	if len(args) > 0 && args[0] == "list" {
		rest = args[1:]
	} else if len(args) > 0 {
		fmt.Fprintf(env.Stderr, "unknown metrics subcommand %q\n", args[0])
		return ExitUsage
	}
	_ = sub

	fs := flag.NewFlagSet("metrics list", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	format := fs.String("format", "table", "output: table|json")
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	reg := metrics.NewDefault()

	if *format == "json" {
		return dumpMetricsJSON(env, reg)
	}
	for _, d := range reg.All() {
		fmt.Fprintf(env.Stdout, "%-20s cost=%d unit=%-16s agg=%-20s rate=%v  %s\n",
			d.ID, d.Cost, d.Unit, d.Aggregation, d.IsRate(), d.Description)
	}
	return ExitOK
}

type metricJSON struct {
	ID          string   `json:"id"`
	Aliases     []string `json:"aliases,omitempty"`
	Unit        string   `json:"unit"`
	Scope       string   `json:"scope"`
	Cost        int      `json:"cost"`
	Aggregation string   `json:"aggregation"`
	Rate        bool     `json:"rate"`
	Description string   `json:"description"`
}

func dumpMetricsJSON(env Env, reg *metrics.Registry) int {
	out := make([]metricJSON, 0)
	for _, d := range reg.All() {
		out = append(out, metricJSON{
			ID: string(d.ID), Aliases: d.Aliases, Unit: string(d.Unit), Scope: string(d.Scope),
			Cost: int(d.Cost), Aggregation: string(d.Aggregation), Rate: d.IsRate(), Description: d.Description,
		})
	}
	enc := json.NewEncoder(env.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	return ExitOK
}

// cmdCapabilities implements `procfit capabilities` (RFC §14.3, §20.2).
func cmdCapabilities(env Env, args []string) int {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	format := fs.String("format", "table", "output: table|json")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	caps := probeCapabilities()
	if *format == "json" {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(caps); err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
			return ExitRuntime
		}
		return ExitOK
	}
	for _, c := range caps {
		fmt.Fprintf(env.Stdout, "%-16s %-14s %s\n", c.Name, c.Status, c.Detail)
	}
	return ExitOK
}

type capability struct {
	Name   string `json:"name"`
	Status string `json:"status"` // available|unsupported|permission_denied
	Detail string `json:"detail"`
}

func probeCapabilities() []capability {
	var caps []capability
	a, err := newAssemblyFn()
	if err != nil {
		return []capability{{Name: "procfs", Status: "unsupported", Detail: err.Error()}}
	}
	stats, err := a.src.List(context.Background())
	switch {
	case err != nil:
		caps = append(caps, capability{"procfs", "permission_denied", err.Error()})
	default:
		caps = append(caps, capability{"procfs", "available", fmt.Sprintf("%d processes visible", len(stats))})
	}
	// Detect whether /proc/PID/io is readable for our own visible processes.
	ioStatus, ioDetail := "unsupported", "no io counters readable"
	for _, s := range stats {
		if s.IOAvail == "available" {
			ioStatus, ioDetail = "available", "per-process io counters readable"
			break
		}
		if s.IOAvail == "permission_denied" {
			ioStatus, ioDetail = "permission_denied", "io counters restricted for some processes"
		}
	}
	caps = append(caps, capability{"proc-io", ioStatus, ioDetail})
	return caps
}
