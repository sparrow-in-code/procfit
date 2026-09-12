//go:build linux

package procfs

import (
	"context"
	"encoding/binary"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/netikras/procfit/internal/model"
)

// perfWindow is the measurement window for one batch of hardware counters. perf
// counters are cumulative, so we enable a group, wait, and read the delta.
const perfWindow = 80 * time.Millisecond

// maxPerfGroups bounds simultaneously-open perf event groups (each uses several
// fds) so we never exhaust RLIMIT_NOFILE (RFC §22). Processes are measured in
// batches of this size.
const maxPerfGroups = 64

// PerfCollector reads hardware performance counters via perf_event_open (RFC
// §13.4). It is opt-in and, without root/CAP_PERFMON (or a permissive
// perf_event_paranoid), degrades to unavailable rather than zero.
type PerfCollector struct {
	window time.Duration
}

// NewPerfCollector builds a perf collector.
func NewPerfCollector() *PerfCollector { return &PerfCollector{window: perfWindow} }

// ID identifies the collector (matches Descriptor.Collector).
func (c *PerfCollector) ID() string { return "perf" }

// Metrics lists the produced metric ids.
func (c *PerfCollector) Metrics() []model.MetricID {
	return []model.MetricID{"cycles", "instructions", "ipc", "cache-misses"}
}

// Collect measures counters for all processes in bounded batches.
func (c *PerfCollector) Collect(ctx context.Context, procs []model.Process) {
	for start := 0; start < len(procs); start += maxPerfGroups {
		if ctx.Err() != nil {
			return
		}
		end := start + maxPerfGroups
		if end > len(procs) {
			end = len(procs)
		}
		c.measureBatch(procs[start:end])
	}
}

type perfGroup struct {
	p      *model.Process
	leader int
	fds    []int
	ok     bool
}

// measureBatch opens counter groups for a batch, runs one shared window, reads
// the deltas, and closes everything.
func (c *PerfCollector) measureBatch(procs []model.Process) {
	groups := make([]perfGroup, len(procs))
	for i := range procs {
		groups[i] = c.openGroup(&procs[i])
	}
	defer func() {
		for _, g := range groups {
			for _, fd := range g.fds {
				_ = unix.Close(fd)
			}
		}
	}()
	for _, g := range groups {
		if g.ok {
			enableGroup(g.leader)
		}
	}
	time.Sleep(c.window)
	secs := c.window.Seconds()
	for _, g := range groups {
		if !g.ok {
			continue
		}
		disableGroup(g.leader)
		c.readGroup(g, secs)
	}
}

// hwEvents is the ordered set of counters in each group (leader first).
var hwEvents = []uint64{
	unix.PERF_COUNT_HW_CPU_CYCLES,
	unix.PERF_COUNT_HW_INSTRUCTIONS,
	unix.PERF_COUNT_HW_CACHE_MISSES,
}

func (c *PerfCollector) openGroup(p *model.Process) perfGroup {
	g := perfGroup{p: p, leader: -1}
	for i, cfg := range hwEvents {
		attr := &unix.PerfEventAttr{
			Type:        unix.PERF_TYPE_HARDWARE,
			Size:        uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
			Config:      cfg,
			Read_format: unix.PERF_FORMAT_GROUP,
		}
		groupFd := -1
		if i == 0 {
			attr.Bits = unix.PerfBitDisabled // leader starts disabled
		} else {
			groupFd = g.leader
		}
		fd, err := unix.PerfEventOpen(attr, p.PID, -1, groupFd, 0)
		if err != nil {
			c.markUnavailable(p, err)
			for _, f := range g.fds {
				_ = unix.Close(f)
			}
			return perfGroup{p: p, leader: -1}
		}
		if i == 0 {
			g.leader = fd
		}
		g.fds = append(g.fds, fd)
	}
	g.ok = true
	return g
}

func (c *PerfCollector) markUnavailable(p *model.Process, err error) {
	avail := model.Unsupported
	if err == unix.EACCES || err == unix.EPERM {
		avail = model.PermissionDenied
	}
	for _, id := range c.Metrics() {
		p.SetMetric(id, model.Unavailable[float64](avail, "perf"))
	}
}

func (c *PerfCollector) readGroup(g perfGroup, secs float64) {
	buf := make([]byte, 8*(1+len(hwEvents)))
	n, err := unix.Read(g.leader, buf)
	if err != nil || n < len(buf) {
		c.markUnavailable(g.p, unix.EIO)
		return
	}
	// Layout: u64 nr; u64 values[nr] (leader-order).
	cycles := float64(binary.LittleEndian.Uint64(buf[8:16]))
	instr := float64(binary.LittleEndian.Uint64(buf[16:24]))
	misses := float64(binary.LittleEndian.Uint64(buf[24:32]))
	set := func(id string, v float64) {
		g.p.SetMetric(model.MetricID(id), model.NewValue(v, model.Sampled, "perf"))
	}
	if secs > 0 {
		set("cycles", cycles/secs)
		set("instructions", instr/secs)
		set("cache-misses", misses/secs)
	}
	ipc := 0.0
	if cycles > 0 {
		ipc = instr / cycles
	}
	set("ipc", ipc)
}

func enableGroup(leader int) {
	_ = unix.IoctlSetInt(leader, unix.PERF_EVENT_IOC_RESET, unix.PERF_IOC_FLAG_GROUP)
	_ = unix.IoctlSetInt(leader, unix.PERF_EVENT_IOC_ENABLE, unix.PERF_IOC_FLAG_GROUP)
}

func disableGroup(leader int) {
	_ = unix.IoctlSetInt(leader, unix.PERF_EVENT_IOC_DISABLE, unix.PERF_IOC_FLAG_GROUP)
}
