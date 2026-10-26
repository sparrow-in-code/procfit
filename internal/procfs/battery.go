package procfs

import (
	"context"
	"os"
	"path/filepath"

	"github.com/netikras/procfit/internal/model"
)

// BatteryCollector reads laptop battery state from /sys/class/power_supply/*
// (type=Battery): charge level, time left, and remaining charge/energy. It is
// vendor/arch-neutral (Intel/AMD/ARM). Multiple batteries are combined. Drivers
// report either charge (µAh) or energy (µWh) — whichever is present is used, the
// other renders unavailable. No battery → everything unavailable, never zero.
// Power draw itself is power-system (PowerCollector).
type BatteryCollector struct{ root string }

// NewBatteryCollector builds the collector rooted at the given sysfs path.
func NewBatteryCollector(root string) *BatteryCollector {
	if root == "" {
		root = "/sys"
	}
	return &BatteryCollector{root: root}
}

// ID names the source.
func (c *BatteryCollector) ID() string { return "battery" }

var batteryIDs = []model.MetricID{
	"battery-state", "battery-pct", "battery-time-left", "battery-charge-left", "battery-energy-left",
}

// Metrics lists the produced ids.
func (c *BatteryCollector) Metrics() []model.MetricID { return batteryIDs }

// batTotals accumulates readings across all batteries. A *Ok flag is set only
// when every battery reported that field, so a partial sum is never shown.
type batTotals struct {
	n                                   int
	chargeNow, chargeFull, current      float64 // µAh, µAh, µA
	energyNow, energyFull, power        float64 // µWh, µWh, µW
	chargeOk, energyOk, currentOk, pwOk bool
	capSum                              float64 // sum of capacity % (fallback level)
	capOk                               bool
	charging, discharging               bool
	states                              []model.BatteryState // per installed battery
	missing                             int                  // battery slots with present=0
}

// CollectHost reads every battery and derives the combined metrics.
func (c *BatteryCollector) CollectHost(_ context.Context) map[model.MetricID]model.MetricValue {
	t, ok := c.readAll()
	if !ok {
		out := allUnavailable(batteryIDs, model.Unsupported, "battery")
		if t.missing > 0 { // a slot exists but the battery is pulled
			out["battery-state"] = stateValue(model.BatteryMissing)
		}
		return out
	}
	return t.metrics()
}

func stateValue(s model.BatteryState) model.MetricValue {
	return model.NewValue(float64(s), model.Exact, "battery")
}

func (c *BatteryCollector) readAll() (batTotals, bool) {
	t := batTotals{chargeOk: true, energyOk: true, currentOk: true, pwOk: true, capOk: true}
	base := filepath.Join(c.root, "class/power_supply")
	entries, err := os.ReadDir(base)
	if err != nil {
		return t, false
	}
	for _, e := range entries {
		dir := filepath.Join(base, e.Name())
		if readTrimFile(filepath.Join(dir, "type")) != "Battery" {
			continue
		}
		if readTrimFile(filepath.Join(dir, "present")) == "0" {
			t.missing++ // empty bay: no readings to fold in
			continue
		}
		t.add(dir)
	}
	return t, t.n > 0
}

// add folds one battery directory into the totals.
func (t *batTotals) add(dir string) {
	t.n++
	t.states = append(t.states, model.BatteryStateFromStatus(readTrimFile(filepath.Join(dir, "status"))))
	t.chargeOk = addField(&t.chargeNow, dir, "charge_now", t.chargeOk)
	t.chargeOk = addField(&t.chargeFull, dir, "charge_full", t.chargeOk)
	t.currentOk = addField(&t.current, dir, "current_now", t.currentOk)
	t.energyOk = addField(&t.energyNow, dir, "energy_now", t.energyOk)
	t.energyOk = addField(&t.energyFull, dir, "energy_full", t.energyOk)
	t.pwOk = addField(&t.power, dir, "power_now", t.pwOk)
	t.capOk = addField(&t.capSum, dir, "capacity", t.capOk)
	switch readTrimFile(filepath.Join(dir, "status")) {
	case "Charging":
		t.charging = true
	case "Discharging":
		t.discharging = true
	}
}

// addField adds a numeric sysfs field to *sum; it returns ok && fieldPresent so a
// field missing on any battery invalidates the combined value.
func addField(sum *float64, dir, name string, ok bool) bool {
	v, present := readUintFile(filepath.Join(dir, name))
	if !present {
		return false
	}
	*sum += float64(v)
	return ok
}

// metrics converts the totals into metric values.
func (t batTotals) metrics() map[model.MetricID]model.MetricValue {
	out := map[model.MetricID]model.MetricValue{
		"battery-state":       stateValue(t.state()),
		"battery-pct":         t.level(),
		"battery-time-left":   t.timeLeft(),
		"battery-charge-left": unavailable("battery"),
		"battery-energy-left": unavailable("battery"),
	}
	if t.chargeOk {
		out["battery-charge-left"] = model.NewValue(t.chargeNow/1000, model.Exact, "battery") // µAh → mAh
	}
	if t.energyOk {
		out["battery-energy-left"] = model.NewValue(t.energyNow/1e6, model.Exact, "battery") // µWh → Wh
	}
	return out
}

// state combines the installed batteries' states: the shared one if they all
// agree, otherwise mixed — with one exception: a discharging battery (or a
// charging one) dominates idle/full ones, since that's what the time-left and
// draw figures describe.
func (t batTotals) state() model.BatteryState {
	if len(t.states) == 0 {
		return model.BatteryUnknown
	}
	first, same := t.states[0], true
	for _, s := range t.states[1:] {
		same = same && s == first
	}
	switch {
	case same:
		return first
	case t.discharging && !t.charging:
		return model.BatteryDischarging
	case t.charging && !t.discharging:
		return model.BatteryCharging
	default:
		return model.BatteryMixed
	}
}

// level is now/full across batteries (charge or energy), else the mean capacity %.
func (t batTotals) level() model.MetricValue {
	switch {
	case t.chargeOk && t.chargeFull > 0:
		return model.NewValue(t.chargeNow/t.chargeFull*100, model.Derived, "battery")
	case t.energyOk && t.energyFull > 0:
		return model.NewValue(t.energyNow/t.energyFull*100, model.Derived, "battery")
	case t.capOk:
		return model.NewValue(t.capSum/float64(t.n), model.Exact, "battery")
	}
	return unavailable("battery")
}

// timeLeft is seconds to empty (discharging) or to full (charging), from the
// remaining (or missing) charge over the current draw — charge/current or
// energy/power, whichever pair the driver reports. Full/idle/zero draw → unavailable.
func (t batTotals) timeLeft() model.MetricValue {
	if !t.charging && !t.discharging {
		return model.Unavailable[float64](model.WarmingUp, "battery") // full / not charging
	}
	var have, rate float64
	switch {
	case t.chargeOk && t.currentOk && t.current > 0:
		have, rate = t.remaining(t.chargeNow, t.chargeFull), t.current
	case t.energyOk && t.pwOk && t.power > 0:
		have, rate = t.remaining(t.energyNow, t.energyFull), t.power
	default:
		return unavailable("battery")
	}
	return model.NewValue(have/rate*3600, model.Derived, "battery") // hours → seconds
}

// remaining is what is left to drain (discharging) or to fill (charging).
func (t batTotals) remaining(now, full float64) float64 {
	if t.charging {
		if full > now {
			return full - now
		}
		return 0
	}
	return now
}

func unavailable(src string) model.MetricValue {
	return model.Unavailable[float64](model.Unsupported, src)
}

// allUnavailable builds a host map with every id unavailable for reason.
func allUnavailable(ids []model.MetricID, reason model.Availability, src string) map[model.MetricID]model.MetricValue {
	out := make(map[model.MetricID]model.MetricValue, len(ids))
	for _, id := range ids {
		out[id] = model.Unavailable[float64](reason, src)
	}
	return out
}
