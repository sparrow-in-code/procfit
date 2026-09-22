package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/render"
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
	setupUsage(env, fs, "metrics list", "list metrics (id, cost, unit, aggregation)",
		"procfit metrics list                # human table of all metrics",
		"procfit metrics list --format json  # machine-readable metric registry")
	if err := fs.Parse(rest); err != nil {
		return parseExit(err)
	}
	reg := metrics.NewDefault()

	dims := query.NewDimensions()
	if *format == "json" {
		return dumpFieldsJSON(env, reg, dims)
	}
	printMetricsSection(env, reg)
	printColumnsSection(env)
	printDimensionsSection(env, dims)
	return ExitOK
}

// printMetricsSection lists the numeric metrics — values usable as columns, sort
// keys, and filter fields. Sourced from the metric registry (single source of
// truth: add a descriptor there and it appears here and everywhere else).
func printMetricsSection(env Env, reg *metrics.Registry) {
	fmt.Fprintln(env.Stdout, "METRICS  (values — usable as columns, --sort, --having):")
	for _, d := range reg.All() {
		fmt.Fprintf(env.Stdout, "  %-20s cost=%d unit=%-16s agg=%-20s rate=%v  %s\n",
			d.ID, d.Cost, d.Unit, d.Aggregation, d.IsRate(), d.Description)
	}
}

// printColumnsSection lists the structural (identity/state) columns from the
// render registry, tagging what each can also be used for.
func printColumnsSection(env Env) {
	fmt.Fprintln(env.Stdout, "\nCOLUMNS  (identity/state fields — usable as columns; tag: [g]roup-by [s]ort [f]ilter):")
	for _, c := range render.StructuralColumns() {
		fmt.Fprintf(env.Stdout, "  %-20s %-6s %s\n", c.ID, columnUses(c), c.Header)
	}
}

// printDimensionsSection lists the --group-by axes from the dimension registry,
// marking those that are also a column with '*'.
func printDimensionsSection(env Env, dims *query.Dimensions) {
	cols := columnIDSet()
	fmt.Fprintln(env.Stdout, "\nDIMENSIONS  (--group-by axes; * = also a column above):")
	for _, id := range dims.IDs() {
		mark := ""
		if cols[id] {
			mark = "*"
		}
		fmt.Fprintf(env.Stdout, "  %-20s %s\n", id+mark, dimensionLabel(dims, id))
	}
}

func columnUses(c render.Column) string {
	s := ""
	if c.Groupable {
		s += "g"
	}
	if c.Sortable {
		s += "s"
	}
	if c.Filterable {
		s += "f"
	}
	if s == "" {
		return "-"
	}
	return "[" + s + "]"
}

func columnIDSet() map[string]bool {
	set := make(map[string]bool)
	for _, c := range render.StructuralColumns() {
		set[c.ID] = true
	}
	return set
}

func dimensionLabel(dims *query.Dimensions, id string) string {
	if d, ok := dims.Get(id); ok {
		return d.Label
	}
	return ""
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

type columnJSON struct {
	ID         string `json:"id"`
	Header     string `json:"header"`
	Sortable   bool   `json:"sortable"`
	Filterable bool   `json:"filterable"`
	Groupable  bool   `json:"groupable"`
}

type dimensionJSON struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	AlsoColumn bool   `json:"also_column"`
}

// fieldsJSON is the machine-readable field catalog: every id usable in
// --columns/--group-by/--sort/--having, split by kind.
type fieldsJSON struct {
	Metrics    []metricJSON    `json:"metrics"`
	Columns    []columnJSON    `json:"columns"`
	Dimensions []dimensionJSON `json:"dimensions"`
}

func dumpFieldsJSON(env Env, reg *metrics.Registry, dims *query.Dimensions) int {
	out := fieldsJSON{
		Metrics:    metricsToJSON(reg),
		Columns:    columnsToJSON(),
		Dimensions: dimensionsToJSON(dims),
	}
	enc := json.NewEncoder(env.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(env.Stderr, "%v\n", err)
		return ExitRuntime
	}
	return ExitOK
}

func metricsToJSON(reg *metrics.Registry) []metricJSON {
	out := make([]metricJSON, 0)
	for _, d := range reg.All() {
		out = append(out, metricJSON{
			ID: string(d.ID), Aliases: d.Aliases, Unit: string(d.Unit), Scope: string(d.Scope),
			Cost: int(d.Cost), Aggregation: string(d.Aggregation), Rate: d.IsRate(), Description: d.Description,
		})
	}
	return out
}

func columnsToJSON() []columnJSON {
	out := make([]columnJSON, 0)
	for _, c := range render.StructuralColumns() {
		out = append(out, columnJSON{
			ID: c.ID, Header: c.Header, Sortable: c.Sortable, Filterable: c.Filterable, Groupable: c.Groupable,
		})
	}
	return out
}

func dimensionsToJSON(dims *query.Dimensions) []dimensionJSON {
	cols := columnIDSet()
	out := make([]dimensionJSON, 0)
	for _, id := range dims.IDs() {
		out = append(out, dimensionJSON{ID: id, Label: dimensionLabel(dims, id), AlsoColumn: cols[id]})
	}
	return out
}

// cmdCapabilities implements `procfit capabilities` (RFC §14.3, §20.2).
func cmdCapabilities(env Env, args []string) int {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	format := fs.String("format", "table", "output: table|json")
	setupUsage(env, fs, "capabilities", "report available/missing collectors and controllers",
		"procfit capabilities                # what this host/kernel/privileges allow",
		"procfit capabilities --format json  # machine-readable capability report")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
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
	caps = append(caps, probeCgroup(), probePerf(), probeEBPF())
	return caps
}

func probeCgroup() capability {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return capability{"cgroup-v2", "available", "unified hierarchy present; freeze/CPU/IO need delegation"}
	}
	return capability{"cgroup-v2", "unsupported", "unified cgroup v2 hierarchy not found"}
}

func probePerf() capability {
	data, err := os.ReadFile("/proc/sys/kernel/perf_event_paranoid")
	if err != nil {
		return capability{"perf-counters", "unsupported", "perf_event_paranoid not readable"}
	}
	level := strings.TrimSpace(string(data))
	if level == "-1" || level == "0" || level == "1" {
		return capability{"perf-counters", "available", "perf_event_paranoid=" + level}
	}
	return capability{"perf-counters", "permission_denied", "perf_event_paranoid=" + level + " (lower it or grant CAP_PERFMON)"}
}

func probeEBPF() capability {
	// No eBPF backend is compiled into this build (RFC §30.5 is still open), so
	// event-traced metrics remain unavailable rather than fabricated.
	return capability{"ebpf", "unsupported", "no eBPF backend in this build; wakeups/net metrics unavailable"}
}
