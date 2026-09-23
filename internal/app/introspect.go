package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/netikras/procfit/internal/metrics"
	"github.com/netikras/procfit/internal/procfs"
	"github.com/netikras/procfit/internal/query"
	"github.com/netikras/procfit/internal/queryspec"
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

	fields := buildFieldCatalog(reg, query.NewDimensions())
	if *format == "json" {
		return dumpFieldsJSON(env, fields)
	}
	printFieldCatalog(env, fields)
	return ExitOK
}

// fieldRow is one entry in the unified field catalog: any id usable in
// --columns/--group-by/--sort/--having, merged across the metric, column, and
// dimension registries by id, so each field appears once with its capabilities.
type fieldRow struct {
	ID         string
	Column     bool // usable in --columns
	Group      bool // usable in --group-by
	Sortable   bool
	Filterable bool
	Metric     bool
	Unit       string
	Cost       int
	Desc       string
}

// buildFieldCatalog merges the three registries (metrics, structural columns,
// group-by dimensions) into one id-keyed catalog. Each registry is the single
// source of truth for its kind; this only unions them, so nothing drifts.
func buildFieldCatalog(reg *metrics.Registry, dims *query.Dimensions) []fieldRow {
	rows := map[string]*fieldRow{}
	get := func(id string) *fieldRow {
		if r, ok := rows[id]; ok {
			return r
		}
		r := &fieldRow{ID: id}
		rows[id] = r
		return r
	}
	for _, d := range reg.All() { // metrics: displayable + sortable values
		r := get(string(d.ID))
		r.Column, r.Sortable, r.Metric = true, true, true
		r.Unit, r.Cost, r.Desc = string(d.Unit), int(d.Cost), d.Description
	}
	for _, c := range render.StructuralColumns() { // structural (identity/state)
		r := get(c.ID)
		r.Column = true
		r.Sortable = r.Sortable || c.Sortable
		if r.Desc == "" {
			r.Desc = c.Desc
		}
	}
	for _, id := range dims.IDs() { // group-by axes
		r := get(id)
		r.Group = true
		if d, ok := dims.Get(id); ok && r.Desc == "" {
			r.Desc = d.Desc
		}
	}
	// Filterability is authoritative from the expr allow-lists.
	filt := filterableFields(reg)
	for id, r := range rows {
		r.Filterable = filt[id]
	}
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]fieldRow, 0, len(ids))
	for _, id := range ids {
		out = append(out, *rows[id])
	}
	return out
}

// filterableFields is the union of the select and having allow-lists — the
// authoritative set of fields usable in --select/--having.
func filterableFields(reg *metrics.Registry) map[string]bool {
	out := map[string]bool{}
	for f := range queryspec.AllowedEntityFields(reg) {
		out[f] = true
	}
	for f := range queryspec.AllowedRowFields(reg) {
		out[f] = true
	}
	return out
}

func printFieldCatalog(env Env, fields []fieldRow) {
	fmt.Fprintln(env.Stdout, "USE flags: c=column  g=group-by  s=sort  f=filter")
	fmt.Fprintf(env.Stdout, "%-22s %-4s %-16s %-4s %s\n", "ID", "USE", "UNIT", "COST", "DESCRIPTION")
	for _, r := range fields {
		fmt.Fprintf(env.Stdout, "%-22s %-4s %-16s %-4s %s\n",
			r.ID, useFlags(r), orDash(r.Unit), costCell(r), r.Desc)
	}
}

// useFlags renders the fixed-position c/g/s/f capability markers ('-' when absent).
func useFlags(r fieldRow) string {
	b := []byte("----")
	if r.Column {
		b[0] = 'c'
	}
	if r.Group {
		b[1] = 'g'
	}
	if r.Sortable {
		b[2] = 's'
	}
	if r.Filterable {
		b[3] = 'f'
	}
	return string(b)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func costCell(r fieldRow) string {
	if !r.Metric {
		return "-"
	}
	return strconv.Itoa(r.Cost)
}

type fieldJSON struct {
	ID          string `json:"id"`
	Column      bool   `json:"column"`
	GroupBy     bool   `json:"group_by"`
	Sortable    bool   `json:"sortable"`
	Filterable  bool   `json:"filterable"`
	Metric      bool   `json:"metric"`
	Unit        string `json:"unit,omitempty"`
	Cost        int    `json:"cost,omitempty"`
	Description string `json:"description,omitempty"`
}

func dumpFieldsJSON(env Env, fields []fieldRow) int {
	out := make([]fieldJSON, 0, len(fields))
	for _, r := range fields {
		out = append(out, fieldJSON{
			ID: r.ID, Column: r.Column, GroupBy: r.Group, Sortable: r.Sortable,
			Filterable: r.Filterable, Metric: r.Metric, Unit: r.Unit, Cost: r.Cost, Description: r.Desc,
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
	caps = append(caps, probeCgroup(), probePerf(), probeEBPF(), probeSchedWakeups(), probePower(), probePSI())
	return caps
}

// probePSI reports whether the kernel exposes pressure-stall information
// (/proc/pressure/cpu, needs CONFIG_PSI) — the host pressure metrics (PM-0514).
func probePSI() capability {
	if _, err := os.Stat("/proc/pressure/cpu"); err == nil {
		return capability{"psi", "available", "pressure-stall info at /proc/pressure/{cpu,io,memory}"}
	}
	return capability{"psi", "unsupported", "/proc/pressure absent (CONFIG_PSI off)"}
}

// probePower reports the active power/energy backend (PM-0506): the first of
// RAPL powercap, hwmon, or battery that yields a reading, else unsupported.
func probePower() capability {
	backend := procfs.NewPowerCollector("").Backend()
	if backend == "" {
		return capability{"power", "unsupported", "no RAPL/hwmon/battery power source on this host"}
	}
	return capability{"power", "available", "power draw via " + backend}
}

// probeSchedWakeups reports the no-root fallback source for `wakeups`: whether
// /proc/self/sched exposes nr_wakeups (needs CONFIG_SCHEDSTATS). eBPF, when
// available, is preferred; this is what backs wakeups without it (PM-0509).
func probeSchedWakeups() capability {
	data, err := os.ReadFile("/proc/self/sched")
	if err != nil {
		return capability{"wakeups-procfs", "unsupported", "/proc/PID/sched not readable"}
	}
	if strings.Contains(string(data), "nr_wakeups") {
		return capability{"wakeups-procfs", "available", "per-process wakeups via /proc/PID/sched (used unless eBPF is active)"}
	}
	return capability{"wakeups-procfs", "unsupported", "nr_wakeups absent (CONFIG_SCHEDSTATS off)"}
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
