package metrics

import (
	"fmt"
	"sort"

	"github.com/netikras/procfit/internal/model"
)

// ProfileName is a named convenience set of metrics (RFC §13.5). Profiles are
// pure data over the registry, not special code paths.
type ProfileName string

const (
	ProfileNone    ProfileName = "none"
	ProfileLight   ProfileName = "light"
	ProfileProcess ProfileName = "process"
	ProfileMemory  ProfileName = "memory"
	ProfileIO      ProfileName = "io"
	ProfileNetwork ProfileName = "network"
	ProfilePower   ProfileName = "power"
	ProfileBattery ProfileName = "battery"
	ProfileSysload ProfileName = "sysload"
	ProfilePerf    ProfileName = "perf"
	ProfileAll     ProfileName = "all"
)

// KnownProfiles lists the built-in profile names.
var KnownProfiles = []ProfileName{
	ProfileNone, ProfileLight, ProfileProcess, ProfileMemory, ProfileIO,
	ProfileNetwork, ProfilePower, ProfileBattery, ProfileSysload, ProfilePerf, ProfileAll,
}

// Profile is a named convenience bundle: the metrics it selects plus optional
// view defaults (explicit ordered columns, sort, group-by, having) that apply
// when the user/config leave those unset. Columns may include non-metric fields
// (e.g. pstate, wchan). Metrics-only profiles omit the view fields.
type Profile struct {
	Metrics []model.MetricID
	Columns []string // explicit ordered display columns; empty => identity + metrics
	Sort    string   // default sort, e.g. "runq-delay:desc"
	GroupBy string   // default grouping
	Having  string   // default filter
}

// profileSpecs maps explicit profiles to their metrics + view defaults. Profiles
// whose membership is derived (none/all/light) are handled in Resolve.
var profileSpecs = map[ProfileName]Profile{
	ProfileProcess: {Metrics: []model.MetricID{"cpu", "rss", "vsz", "threads", "pnice", "pstate"}},
	// memory: what a process actually costs — proportional/unique set size split
	// out from shared, plus swap, peak, the anon/file breakdown and OOM risk.
	ProfileMemory: {
		Metrics: []model.MetricID{"rss", "pss", "uss", "swap", "mem-peak", "rss-anon", "oom-score", "major-faults"},
		Columns: []string{"target", "pid", "rss", "pss", "uss", "swap", "mem-peak", "rss-anon", "oom-score", "major-faults"},
		Sort:    "pss:desc",
	},
	ProfileIO:      {Metrics: []model.MetricID{"disk-rbps", "disk-wbps", "io-rchar", "io-wchar", "read-syscalls", "write-syscalls"}},
	ProfileNetwork: {Metrics: []model.MetricID{"net-rx-bps", "net-tx-bps", "net-rx-pps", "net-tx-pps"}},
	// power: actual consumption — real watts (RAPL/hwmon/battery) plus deep-idle
	// residency and cpu for context. battery (below) stays the per-process drain
	// DRIVERS (wakeups/ctxsw/cpu/gpu), so the two profiles answer different
	// questions: how much is drawn vs what is causing the draw.
	ProfilePower: {Metrics: []model.MetricID{"power-pkg", "power-core", "power-dram", "power-system", "cstate-deep-residency", "cpu"}},
	// battery: what actually drains a laptop. Wakeups (eBPF, needs privilege) are
	// the real signal — a process can be ~0% cpu yet keep the package out of deep
	// C-states. ctxsw-voluntary is a light, no-root proxy for that wake/sleep
	// churn, so the profile still says something useful without eBPF; cpu-normalized
	// frames cpu against total host capacity.
	ProfileBattery: {Metrics: []model.MetricID{"cpu", "cpu-normalized", "wakeups", "timer-wakeups", "ctxsw-voluntary", "gpu"}},
	// sysload: decompose load average — state + the blocking cause (wchan), CPU use,
	// CPU-wait (runq), I/O-wait (blkio), preemption churn, page-in thrash. Sorted by
	// runq-delay so the CPU-starved tasks surface first.
	ProfileSysload: {
		Metrics: []model.MetricID{"cpu", "runq-delay", "blkio-delay", "ctxsw-involuntary", "major-faults"},
		Columns: []string{"target", "pid", "pstate", "wchan", "cpu", "runq-delay", "blkio-delay", "ctxsw-involuntary", "major-faults"},
		Sort:    "runq-delay:desc",
	},
	ProfilePerf: {Metrics: []model.MetricID{"cycles", "instructions", "ipc", "cache-references", "cache-misses"}},
}

// ProfileDefaults returns an explicit profile's view defaults (metrics + columns/
// sort/group-by/having); ok=false for derived (none/all/light) or unknown names.
func ProfileDefaults(name ProfileName) (Profile, bool) {
	p, ok := profileSpecs[name]
	return p, ok
}

// ProfileNames returns the built-in profile names in display order (for help
// text), derived from KnownProfiles so it never drifts from the flag docs.
func ProfileNames() []string {
	out := make([]string, len(KnownProfiles))
	for i, p := range KnownProfiles {
		out[i] = string(p)
	}
	return out
}

// IsKnownProfile reports whether name is a recognized profile. It is derived from
// the actual profile specs (plus the registry-derived none/light/all), so a
// profile added to profileSpecs is recognized without touching a second list.
func IsKnownProfile(name string) bool {
	switch ProfileName(name) {
	case ProfileNone, ProfileLight, ProfileAll:
		return true
	}
	_, ok := profileSpecs[ProfileName(name)]
	return ok
}

// Resolve expands a profile into the set of canonical metric ids that exist in
// the registry. Ids referenced by a profile but not present in the registry are
// silently skipped (they may be compiled out); "all" and "light" derive their
// membership from the registry directly.
func (r *Registry) Resolve(name ProfileName) ([]model.MetricID, error) {
	switch name {
	case ProfileNone:
		return nil, nil
	case ProfileAll:
		return r.IDs(), nil
	case ProfileLight:
		var out []model.MetricID
		for _, d := range r.All() {
			if d.Cost == Cost0Light {
				out = append(out, d.ID)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out, nil
	}
	spec, ok := profileSpecs[name]
	if !ok {
		return nil, fmt.Errorf("unknown metric profile %q", name)
	}
	members := spec.Metrics
	var out []model.MetricID
	for _, id := range members {
		if r.Has(string(id)) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}
