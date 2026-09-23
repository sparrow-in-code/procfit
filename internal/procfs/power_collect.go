package procfs

import (
	"context"
	"sync"
	"time"

	"github.com/netikras/procfit/internal/model"
	"github.com/netikras/procfit/internal/ports"
)

// powerWindow is the self-contained window used to turn cumulative energy
// counters into watts.
const powerWindow = 200 * time.Millisecond

// powerDomains maps a canonical power domain to its metric id.
var powerDomains = map[string]model.MetricID{
	"pkg":    "power-pkg",
	"core":   "power-core",
	"uncore": "power-uncore",
	"dram":   "power-dram",
	"system": "power-system",
}

// PowerCollector derives per-domain watts from the selected PowerSource and
// broadcasts them onto every process (host metrics, RFC §13/§24). Energy-counter
// domains are delta'd over a self-window; instantaneous domains are used
// directly. The active backend (powercap/hwmon/battery) is chosen once. Missing
// domains render unavailable, never zero.
type PowerCollector struct {
	root   string
	window time.Duration
	sleep  func(time.Duration)
	once   sync.Once
	src    ports.PowerSource
}

// NewPowerCollector builds the collector over the given sysfs root.
func NewPowerCollector(root string) *PowerCollector {
	return &PowerCollector{root: root, window: powerWindow, sleep: time.Sleep}
}

// ID names the source family.
func (c *PowerCollector) ID() string { return "power" }

// Metrics lists every power domain metric this collector can produce.
func (c *PowerCollector) Metrics() []model.MetricID {
	return []model.MetricID{"power-pkg", "power-core", "power-uncore", "power-dram", "power-system"}
}

// Backend reports the selected source id (for capabilities), or "" if none.
func (c *PowerCollector) Backend() string {
	c.once.Do(func() { c.src = NewPowerSource(c.root) })
	if c.src == nil {
		return ""
	}
	return c.src.ID()
}

// Collect measures power and broadcasts each domain onto all processes.
func (c *PowerCollector) Collect(_ context.Context, procs []model.Process) {
	c.once.Do(func() { c.src = NewPowerSource(c.root) })
	if c.src == nil {
		c.broadcastAll(procs, model.Unavailable[float64](model.Unsupported, "power"))
		return
	}
	r1, ok := c.src.Read()
	if !ok {
		c.broadcastAll(procs, model.Unavailable[float64](model.Unsupported, "power"))
		return
	}
	watts := c.watts(r1)
	for domain, id := range powerDomains {
		v, ok := watts[domain]
		if !ok {
			v = model.Unavailable[float64](model.Unsupported, c.src.ID())
		}
		c.broadcast(procs, id, v)
	}
}

// watts reduces a set of readings to per-domain watt values, taking a second
// energy sample over the window when any domain is a cumulative counter.
func (c *PowerCollector) watts(r1 []ports.PowerReading) map[string]model.MetricValue {
	energy1 := sumEnergy(r1)
	var energy2 map[string]uint64
	if len(energy1) > 0 {
		c.sleep(c.window)
		if r2, ok := c.src.Read(); ok {
			energy2 = sumEnergy(r2)
		}
	}
	secs := c.window.Seconds()
	out := make(map[string]model.MetricValue, len(powerDomains))
	// Instantaneous domains: microwatts → watts directly.
	for domain, uw := range sumInstant(r1) {
		out[domain] = model.NewValue(float64(uw)/1e6, model.Sampled, c.src.ID())
	}
	// Energy-counter domains: (ΔµJ / 1e6) / secs → watts.
	for domain, uj1 := range energy1 {
		uj2, ok := energy2[domain]
		if !ok || secs <= 0 || uj2 < uj1 { // absent 2nd read or counter wrap
			out[domain] = model.Unavailable[float64](model.WarmingUp, c.src.ID())
			continue
		}
		out[domain] = model.NewValue(float64(uj2-uj1)/1e6/secs, model.Sampled, c.src.ID())
	}
	markUnavailableDomains(out, r1, c.src.ID())
	return out
}

// sumEnergy sums cumulative µJ per domain across sockets (Available only).
func sumEnergy(rs []ports.PowerReading) map[string]uint64 {
	out := map[string]uint64{}
	for _, r := range rs {
		if r.Kind == ports.EnergyCounter && r.Avail == model.Available {
			out[r.Domain] += r.Value
		}
	}
	return out
}

// sumInstant sums instantaneous µW per domain across sensors (Available only).
func sumInstant(rs []ports.PowerReading) map[string]uint64 {
	out := map[string]uint64{}
	for _, r := range rs {
		if r.Kind == ports.InstantPower && r.Avail == model.Available {
			out[r.Domain] += r.Value
		}
	}
	return out
}

// markUnavailableDomains records a reason for domains that were present but not
// Available (e.g. root-only RAPL), so they render "?"/"-", never a fake zero.
func markUnavailableDomains(out map[string]model.MetricValue, rs []ports.PowerReading, src string) {
	for _, r := range rs {
		if r.Avail == model.Available {
			continue
		}
		if _, done := out[r.Domain]; !done {
			out[r.Domain] = model.Unavailable[float64](r.Avail, src)
		}
	}
}

func (c *PowerCollector) broadcast(procs []model.Process, id model.MetricID, v model.MetricValue) {
	for i := range procs {
		procs[i].SetMetric(id, v)
	}
}

func (c *PowerCollector) broadcastAll(procs []model.Process, v model.MetricValue) {
	for _, id := range c.Metrics() {
		c.broadcast(procs, id, v)
	}
}
