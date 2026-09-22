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

// profileMembers maps explicit profiles to canonical metric ids. Profiles whose
// membership is derived (none/all/light) are handled in Resolve.
var profileMembers = map[ProfileName][]model.MetricID{
	ProfileProcess: {"cpu", "rss", "vsz", "threads", "pnice", "pstate"},
	// memory: what a process actually costs — proportional/unique set size split
	// out from shared, plus swap, peak, the anon/file breakdown and OOM risk.
	ProfileMemory:  {"rss", "pss", "uss", "swap", "mem-peak", "rss-anon", "oom-score", "major-faults"},
	ProfileIO:      {"disk-rbps", "disk-wbps", "io-rchar", "io-wchar", "read-syscalls", "write-syscalls"},
	ProfileNetwork: {"net-rx-bps", "net-tx-bps", "net-rx-pps", "net-tx-pps"},
	ProfilePower:   {"cpu", "wakeups", "timer-wakeups", "gpu", "gpu-mem"},
	// battery: what actually drains a laptop. Wakeups (eBPF, needs privilege) are
	// the real signal — a process can be ~0% cpu yet keep the package out of deep
	// C-states. ctxsw-voluntary is a light, no-root proxy for that wake/sleep
	// churn, so the profile still says something useful without eBPF; cpu-normalized
	// frames cpu against total host capacity.
	ProfileBattery: {"cpu", "cpu-normalized", "wakeups", "timer-wakeups", "ctxsw-voluntary", "gpu"},
	// sysload: decompose load average — CPU use, CPU-wait (runq), I/O-wait
	// (blkio), preemption churn, and page-in thrash. Pair with `--columns +pstate`
	// or `--leaf thread`/`--having 'pstate == "D"'` to see the exact R/D tasks.
	ProfileSysload: {"cpu", "runq-delay", "blkio-delay", "ctxsw-involuntary", "major-faults"},
	ProfilePerf:    {"cycles", "instructions", "ipc", "cache-references", "cache-misses"},
}

// IsKnownProfile reports whether name is a recognized profile.
func IsKnownProfile(name string) bool {
	for _, p := range KnownProfiles {
		if ProfileName(name) == p {
			return true
		}
	}
	return false
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
	members, ok := profileMembers[name]
	if !ok {
		return nil, fmt.Errorf("unknown metric profile %q", name)
	}
	var out []model.MetricID
	for _, id := range members {
		if r.Has(string(id)) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}
