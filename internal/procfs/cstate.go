package procfs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/netikras/procfit/internal/model"
)

// cstateWindow is the self-contained measurement window for C-state residency.
const cstateWindow = 200 * time.Millisecond

// CstateCollector computes host-wide deep C-state residency from the cpuidle
// sysfs (/sys/devices/system/cpu/cpu*/cpuidle/state*/time). It self-windows (read
// cumulative residency, wait, read again) and reports the % of CPU-time spent in
// deep idle (states deeper than C1/polling) across all CPUs, broadcast onto every
// process (a host metric, RFC §13/§24). The cpuidle framework is arch-neutral
// (x86/ARM) so no vendor/driver name is assumed — a deeper state index means a
// deeper idle state. Absent cpuidle → unavailable, never zero.
type CstateCollector struct {
	root   string // sysfs root ("/sys"), overridable for tests
	window time.Duration
	sleep  func(time.Duration) // injectable so tests drive the window deterministically
}

// NewCstateCollector builds the collector rooted at the given sysfs path.
func NewCstateCollector(root string) *CstateCollector {
	if root == "" {
		root = "/sys"
	}
	return &CstateCollector{root: root, window: cstateWindow, sleep: time.Sleep}
}

// ID names the source of the produced value.
func (c *CstateCollector) ID() string { return "cstate" }

// Metrics lists the produced ids.
func (c *CstateCollector) Metrics() []model.MetricID {
	return []model.MetricID{"cstate-deep-residency"}
}

// Collect measures deep-idle residency over the window and broadcasts it to all
// processes (a host value; AggMax collapses the identical copies on rollup).
func (c *CstateCollector) Collect(_ context.Context, procs []model.Process) {
	first, cpus, ok := c.readDeep()
	if !ok {
		c.broadcast(procs, model.Unavailable[float64](model.Unsupported, "cstate"))
		return
	}
	c.sleep(c.window)
	second, _, ok := c.readDeep()
	windowUs := float64(c.window.Microseconds()) * float64(cpus)
	if !ok || windowUs <= 0 || second < first {
		c.broadcast(procs, model.Unavailable[float64](model.WarmingUp, "cstate"))
		return
	}
	pct := float64(second-first) / windowUs * 100
	c.broadcast(procs, model.NewValue(pct, model.Sampled, "cstate"))
}

func (c *CstateCollector) broadcast(procs []model.Process, v model.MetricValue) {
	for i := range procs {
		procs[i].SetMetric("cstate-deep-residency", v)
	}
}

// readDeep returns the summed deep-idle residency (microseconds) across all CPUs
// and the CPU count. ok=false when cpuidle is absent entirely.
func (c *CstateCollector) readDeep() (sum uint64, cpus int, ok bool) {
	base := filepath.Join(c.root, "devices/system/cpu")
	entries, err := os.ReadDir(base)
	if err != nil {
		return 0, 0, false
	}
	anyDeep := false
	for _, e := range entries {
		if !isCPUDir(e.Name()) {
			continue
		}
		t, present, hadDeep := cpuDeepTime(filepath.Join(base, e.Name(), "cpuidle"))
		if !present {
			continue
		}
		cpus++
		sum += t
		anyDeep = anyDeep || hadDeep
	}
	if cpus == 0 || !anyDeep {
		return 0, 0, false
	}
	return sum, cpus, true
}

// cpuDeepTime sums the residency (µs) of one CPU's deep idle states (index >= 2,
// i.e. deeper than POLL/C1). present reports the cpuidle dir was readable;
// hadDeep reports at least one deep-state time was read.
func cpuDeepTime(idleDir string) (sum uint64, present, hadDeep bool) {
	states, err := os.ReadDir(idleDir)
	if err != nil {
		return 0, false, false
	}
	for _, s := range states {
		if idx, ok := stateIndex(s.Name()); !ok || idx < 2 {
			continue
		}
		if t, ok := readUintFile(filepath.Join(idleDir, s.Name(), "time")); ok {
			sum += t
			hadDeep = true
		}
	}
	return sum, true, hadDeep
}

// isCPUDir reports whether a name is a per-CPU directory ("cpu0".."cpuN"),
// excluding cpufreq/cpuidle/etc.
func isCPUDir(name string) bool {
	if !strings.HasPrefix(name, "cpu") || len(name) == 3 {
		return false
	}
	_, err := strconv.Atoi(name[3:])
	return err == nil
}

// stateIndex parses a cpuidle "stateN" directory name into its index.
func stateIndex(name string) (int, bool) {
	if !strings.HasPrefix(name, "state") {
		return 0, false
	}
	n, err := strconv.Atoi(name[len("state"):])
	if err != nil {
		return 0, false
	}
	return n, true
}

// readUintFile reads a single unsigned integer from a sysfs file.
func readUintFile(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
