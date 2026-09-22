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

// schedstatWindow is the self-contained measurement window for runq-delay.
const schedstatWindow = 200 * time.Millisecond

// SchedstatCollector derives runq-delay — the % of wall time a task spent
// runnable but waiting for a CPU — from /proc/PID/schedstat (field 2, the
// run-queue wait in ns). Like the GPU/wakeups collectors it self-windows (read,
// wait, read) so a single one-shot enrich still yields a value. It needs
// CONFIG_SCHEDSTATS; without it the file is absent and runq-delay is unavailable.
type SchedstatCollector struct {
	root   string
	window time.Duration
	sleep  func(time.Duration) // injectable so tests drive the window deterministically
}

// NewSchedstatCollector builds the collector rooted at the given procfs path.
func NewSchedstatCollector(root string) *SchedstatCollector {
	if root == "" {
		root = "/proc"
	}
	return &SchedstatCollector{root: root, window: schedstatWindow, sleep: time.Sleep}
}

// ID matches Descriptor.Collector for runq-delay.
func (c *SchedstatCollector) ID() string { return "schedstat" }

// Metrics lists the produced ids.
func (c *SchedstatCollector) Metrics() []model.MetricID { return []model.MetricID{"runq-delay"} }

type schedSample struct {
	ns    uint64
	avail model.Availability
}

// Collect derives runq-delay from the growth of run-queue wait time over the
// window. If nothing is readable (e.g. CONFIG_SCHEDSTATS off) it skips the wait.
func (c *SchedstatCollector) Collect(_ context.Context, procs []model.Process) {
	first := make([]schedSample, len(procs))
	anyOK := false
	for i := range procs {
		ns, av := c.readRunDelay(procs[i].PID)
		first[i] = schedSample{ns, av}
		if av == model.Available {
			anyOK = true
		}
	}
	if !anyOK {
		for i := range procs {
			procs[i].SetMetric("runq-delay", model.Unavailable[float64](first[i].avail, "schedstat"))
		}
		return
	}

	c.sleep(c.window)
	windowNs := float64(c.window.Nanoseconds())
	for i := range procs {
		p := &procs[i]
		if first[i].avail != model.Available {
			p.SetMetric("runq-delay", model.Unavailable[float64](first[i].avail, "schedstat"))
			continue
		}
		ns2, av2 := c.readRunDelay(p.PID)
		if av2 != model.Available || windowNs <= 0 || ns2 < first[i].ns {
			p.SetMetric("runq-delay", model.Unavailable[float64](model.WarmingUp, "schedstat"))
			continue
		}
		pct := float64(ns2-first[i].ns) / windowNs * 100
		p.SetMetric("runq-delay", model.NewValue(pct, model.Sampled, "schedstat"))
	}
}

func (c *SchedstatCollector) readRunDelay(pid int) (uint64, model.Availability) {
	data, err := os.ReadFile(filepath.Join(c.root, itoa(pid), "schedstat"))
	if err != nil {
		switch {
		case os.IsPermission(err):
			return 0, model.PermissionDenied
		case os.IsNotExist(err):
			return 0, model.Unsupported // CONFIG_SCHEDSTATS off, or the task vanished
		default:
			return 0, model.ReadError
		}
	}
	ns, ok := parseSchedstatRunDelay(string(data))
	if !ok {
		return 0, model.ReadError
	}
	return ns, model.Available
}

// parseSchedstatRunDelay returns the run-queue wait (ns, 2nd field) of a
// /proc/PID/schedstat line: "time_on_cpu_ns run_delay_ns timeslices".
func parseSchedstatRunDelay(s string) (uint64, bool) {
	f := strings.Fields(s)
	if len(f) < 2 {
		return 0, false
	}
	n, err := strconv.ParseUint(f[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
