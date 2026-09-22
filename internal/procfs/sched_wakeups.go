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

// schedWakeupsWindow is the self-contained measurement window for the wakeups
// rate (read cumulative nr_wakeups, wait, read again), so a single one-shot
// enrich still yields a value.
const schedWakeupsWindow = 200 * time.Millisecond

// SchedWakeupsCollector derives a per-second `wakeups` rate from
// /proc/PID/sched (nr_wakeups) — a no-root, no-eBPF fallback (PM-0509). It is a
// FALLBACK source: it only fills processes whose `wakeups` the preferred eBPF
// collector did not provide, so eBPF (richer: waker attribution) always wins and
// there is never double counting. Needs CONFIG_SCHEDSTATS; without it nr_wakeups
// is absent and wakeups degrades to unavailable, never zero.
type SchedWakeupsCollector struct {
	root   string
	window time.Duration
	sleep  func(time.Duration) // injectable so tests drive the window deterministically
}

// NewSchedWakeupsCollector builds the collector rooted at the given procfs path.
func NewSchedWakeupsCollector(root string) *SchedWakeupsCollector {
	if root == "" {
		root = "/proc"
	}
	return &SchedWakeupsCollector{root: root, window: schedWakeupsWindow, sleep: time.Sleep}
}

// ID names the source that produced the value (surfaced via capabilities).
func (c *SchedWakeupsCollector) ID() string { return "sched" }

// Metrics lists the produced ids.
func (c *SchedWakeupsCollector) Metrics() []model.MetricID { return []model.MetricID{"wakeups"} }

type schedWakeSample struct {
	n     uint64
	avail model.Availability
}

// Collect fills the wakeups rate for processes the eBPF source left unresolved.
func (c *SchedWakeupsCollector) Collect(_ context.Context, procs []model.Process) {
	pending := c.pending(procs)
	if len(pending) == 0 {
		return // the preferred source supplied everything; no window needed
	}
	first, anyOK := c.firstSamples(procs, pending)
	if !anyOK {
		for _, i := range pending {
			procs[i].SetMetric("wakeups", model.Unavailable[float64](first[i].avail, "sched"))
		}
		return
	}
	c.sleep(c.window)
	for _, i := range pending {
		c.finish(&procs[i], first[i])
	}
}

// pending returns the indices of processes whose wakeups is not already present
// (eBPF is preferred, so those are left untouched).
func (c *SchedWakeupsCollector) pending(procs []model.Process) []int {
	out := make([]int, 0, len(procs))
	for i := range procs {
		if !procs[i].Metric("wakeups").Present() {
			out = append(out, i)
		}
	}
	return out
}

// firstSamples reads the first nr_wakeups sample for each pending process.
func (c *SchedWakeupsCollector) firstSamples(procs []model.Process, pending []int) (map[int]schedWakeSample, bool) {
	first := make(map[int]schedWakeSample, len(pending))
	anyOK := false
	for _, i := range pending {
		n, av := c.readWakeups(procs[i].PID)
		first[i] = schedWakeSample{n, av}
		if av == model.Available {
			anyOK = true
		}
	}
	return first, anyOK
}

// finish reads the second sample and sets the per-second rate (or the reason).
func (c *SchedWakeupsCollector) finish(p *model.Process, s1 schedWakeSample) {
	if s1.avail != model.Available {
		p.SetMetric("wakeups", model.Unavailable[float64](s1.avail, "sched"))
		return
	}
	secs := c.window.Seconds()
	n2, av2 := c.readWakeups(p.PID)
	if av2 != model.Available || secs <= 0 || n2 < s1.n {
		p.SetMetric("wakeups", model.Unavailable[float64](model.WarmingUp, "sched"))
		return
	}
	rate := float64(n2-s1.n) / secs
	p.SetMetric("wakeups", model.NewValue(rate, model.Sampled, "sched"))
}

func (c *SchedWakeupsCollector) readWakeups(pid int) (uint64, model.Availability) {
	data, err := os.ReadFile(filepath.Join(c.root, itoa(pid), "sched"))
	if err != nil {
		switch {
		case os.IsPermission(err):
			return 0, model.PermissionDenied
		case os.IsNotExist(err):
			return 0, model.Unsupported // no sched_debug, or the task vanished
		default:
			return 0, model.ReadError
		}
	}
	n, ok := parseSchedField(string(data), "nr_wakeups")
	if !ok {
		return 0, model.Unsupported // CONFIG_SCHEDSTATS off: the counter is absent
	}
	return n, model.Available
}

// parseSchedField extracts a "key : value" unsigned counter from a
// /proc/PID/sched dump (e.g. nr_wakeups, nr_voluntary_switches).
func parseSchedField(data, key string) (uint64, bool) {
	for _, line := range strings.Split(data, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(k) != key {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}
