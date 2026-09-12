//go:build linux

package procfs

import (
	"bytes"
	"context"
	_ "embed"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"github.com/netikras/procfit/internal/model"
)

// wakeupsObj is the precompiled BPF object (see bpf/wakeups.bpf.c). It is
// embedded so the static binary carries it — no toolchain is needed on the
// target host, only a BTF-less tracepoint load (root/CAP_BPF).
//
//go:embed bpf/wakeups.bpf.o
var wakeupsObj []byte

// WakeupsCollector counts scheduler wakeups per process via an eBPF tracepoint
// (RFC §13.3, waking-task attribution). It loads lazily on first use and, if the
// kernel/privileges do not permit it, degrades to unavailable (never zero).
type WakeupsCollector struct {
	mu     sync.Mutex
	loaded bool
	failed bool
	reason model.Availability

	coll *ebpf.Collection
	link link.Link
	m    *ebpf.Map
}

// wakeupsWindow is the self-contained measurement window: the collector reads the
// cumulative per-tgid map, waits, and reads again to derive a per-second rate,
// so a single one-shot enrich (e.g. `ps`) still yields a real value.
const wakeupsWindow = 200 * time.Millisecond

// NewWakeupsCollector builds the collector (nothing is loaded until first use).
func NewWakeupsCollector() *WakeupsCollector { return &WakeupsCollector{} }

// ID matches Descriptor.Collector for the event metrics.
func (c *WakeupsCollector) ID() string { return "ebpf" }

// Metrics lists the produced ids.
func (c *WakeupsCollector) Metrics() []model.MetricID { return []model.MetricID{"wakeups"} }

// Collect updates the wakeups rate for each process. The first call warms up
// (no prior counter sample), matching the light sampler's rate semantics.
func (c *WakeupsCollector) Collect(_ context.Context, procs []model.Process) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.failed {
		c.markAll(procs, c.reason)
		return
	}
	if !c.loaded {
		if err := c.load(); err != nil {
			c.failed = true
			c.reason = classifyBPFErr(err)
			c.markAll(procs, c.reason)
			return
		}
		c.loaded = true
	}

	c1 := c.readCounts()
	time.Sleep(wakeupsWindow)
	c2 := c.readCounts()
	secs := wakeupsWindow.Seconds()
	for i := range procs {
		p := &procs[i]
		curV, prevV := c2[uint32(p.PID)], c1[uint32(p.PID)]
		if secs <= 0 || curV < prevV {
			p.SetMetric("wakeups", model.Unavailable[float64](model.WarmingUp, "ebpf"))
			continue
		}
		p.SetMetric("wakeups", model.NewValue(float64(curV-prevV)/secs, model.Sampled, "ebpf"))
	}
}

func (c *WakeupsCollector) load() error {
	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(wakeupsObj))
	if err != nil {
		return err
	}
	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return err
	}
	lnk, err := link.Tracepoint("sched", "sched_wakeup", coll.Programs["on_sched_wakeup"], nil)
	if err != nil {
		coll.Close()
		return err
	}
	c.coll, c.link, c.m = coll, lnk, coll.Maps["wakeups"]
	return nil
}

func (c *WakeupsCollector) readCounts() map[uint32]uint64 {
	out := make(map[uint32]uint64)
	if c.m == nil {
		return out
	}
	var k uint32
	var v uint64
	it := c.m.Iterate()
	for it.Next(&k, &v) {
		out[k] = v
	}
	return out
}

func (c *WakeupsCollector) markAll(procs []model.Process, reason model.Availability) {
	for i := range procs {
		procs[i].SetMetric("wakeups", model.Unavailable[float64](reason, "ebpf"))
	}
}

// Close releases the eBPF resources.
func (c *WakeupsCollector) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.link != nil {
		_ = c.link.Close()
	}
	if c.coll != nil {
		c.coll.Close()
	}
}

func classifyBPFErr(err error) model.Availability {
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "permission") || strings.Contains(s, "not permitted") || strings.Contains(s, "operation not permitted") {
		return model.PermissionDenied
	}
	return model.Unsupported
}
