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

// PowerCollector derives per-domain watts by MERGING every available PowerSource
// and broadcasts them onto every process (host metrics, RFC §13/§24).
// Energy-counter domains are delta'd over a self-window; instantaneous domains
// are used directly. Backends are discovered once; a domain no backend provides
// renders unavailable, never zero.
type PowerCollector struct {
	root    string
	window  time.Duration
	sleep   func(time.Duration)
	once    sync.Once
	sources []ports.PowerSource
}

// NewPowerCollector builds the collector over the given sysfs root.
func NewPowerCollector(root string) *PowerCollector {
	return &PowerCollector{root: root, window: powerWindow, sleep: time.Sleep}
}

func (c *PowerCollector) init() {
	c.once.Do(func() { c.sources = NewPowerSources(c.root) })
}

// ID names the source family.
func (c *PowerCollector) ID() string { return "power" }

// Metrics lists every power domain metric this collector can produce.
func (c *PowerCollector) Metrics() []model.MetricID {
	return []model.MetricID{"power-pkg", "power-core", "power-uncore", "power-dram", "power-system"}
}

// Backends reports the active backend ids (for capabilities); empty if none.
func (c *PowerCollector) Backends() []string {
	c.init()
	ids := make([]string, 0, len(c.sources))
	for _, s := range c.sources {
		ids = append(ids, s.ID())
	}
	return ids
}

// CollectHost measures power and returns each domain as a host metric.
func (c *PowerCollector) CollectHost(_ context.Context) map[model.MetricID]model.MetricValue {
	c.init()
	out := make(map[model.MetricID]model.MetricValue, len(powerDomains))
	if len(c.sources) == 0 {
		for _, id := range c.Metrics() {
			out[id] = model.Unavailable[float64](model.Unsupported, "power")
		}
		return out
	}
	merged := c.readMerged()
	for domain, id := range powerDomains {
		if v, ok := merged[domain]; ok {
			out[id] = v
		} else {
			out[id] = model.Unavailable[float64](model.Unsupported, "power")
		}
	}
	return out
}

// readMerged reads all backends (one shared window for the energy counters) and
// merges their domains: the first Available value wins, so a domain one backend
// cannot read is still filled by another that can.
func (c *PowerCollector) readMerged() map[string]model.MetricValue {
	firsts := make([][]ports.PowerReading, len(c.sources))
	needWindow := false
	for i, s := range c.sources {
		if r, ok := s.Read(); ok {
			firsts[i] = r
			needWindow = needWindow || hasEnergy(r)
		}
	}
	if needWindow {
		c.sleep(c.window)
	}
	secs := c.window.Seconds()
	merged := map[string]model.MetricValue{}
	for i, s := range c.sources {
		r1 := firsts[i]
		if r1 == nil {
			continue
		}
		var r2 []ports.PowerReading
		if hasEnergy(r1) {
			r2, _ = s.Read()
		}
		for domain, v := range wattsFrom(r1, r2, secs, s.ID()) {
			mergeDomain(merged, domain, v)
		}
	}
	return merged
}

// mergeDomain keeps the first value seen for a domain, upgrading an unavailable
// placeholder to an Available reading from a later backend.
func mergeDomain(m map[string]model.MetricValue, domain string, v model.MetricValue) {
	if cur, ok := m[domain]; !ok || (!cur.Present() && v.Present()) {
		m[domain] = v
	}
}

func hasEnergy(rs []ports.PowerReading) bool {
	for _, r := range rs {
		if r.Kind == ports.EnergyCounter {
			return true
		}
	}
	return false
}

// wattsFrom reduces one backend's readings to per-domain watt values, using the
// second energy sample (r2) for cumulative counters.
func wattsFrom(r1, r2 []ports.PowerReading, secs float64, src string) map[string]model.MetricValue {
	out := map[string]model.MetricValue{}
	// Instantaneous domains: microwatts → watts directly.
	for domain, uw := range sumInstant(r1) {
		out[domain] = model.NewValue(float64(uw)/1e6, model.Sampled, src)
	}
	// Energy-counter domains: (ΔµJ / 1e6) / secs → watts.
	energy1, energy2 := sumEnergy(r1), sumEnergy(r2)
	for domain, uj1 := range energy1 {
		uj2, ok := energy2[domain]
		if !ok || secs <= 0 || uj2 < uj1 { // absent 2nd read or counter wrap
			out[domain] = model.Unavailable[float64](model.WarmingUp, src)
			continue
		}
		out[domain] = model.NewValue(float64(uj2-uj1)/1e6/secs, model.Sampled, src)
	}
	markUnavailableDomains(out, r1, src)
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
