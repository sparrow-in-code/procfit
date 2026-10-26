package metrics

import "github.com/netikras/procfit/internal/model"

// builtinSys are whole-machine readouts (RFC §13): load average, battery state,
// and temperatures. All are host-scoped — read once per sample, shown in the
// system section, never per process — and cost-1 so the default light view does
// no extra host reads. A value the host does not expose renders unavailable,
// never a fabricated zero.
var builtinSys = []Descriptor{
	hostGauge("load-1", UnitRatio, "loadavg", "host load average over 1 minute (/proc/loadavg)"),
	hostGauge("load-5", UnitRatio, "loadavg", "host load average over 5 minutes"),
	hostGauge("load-15", UnitRatio, "loadavg", "host load average over 15 minutes"),
	hostGauge("battery-state", UnitEnum, "battery",
		"battery state: charging/discharging/full/not-charging/missing/unknown (mixed if batteries disagree)"),
	hostGauge("battery-pct", UnitPercent, "battery", "battery charge level % (all batteries combined)"),
	hostGauge("battery-time-left", UnitDuration, "battery",
		"battery time left: to empty while discharging, to full while charging"),
	hostGauge("battery-charge-left", UnitMilliAmpHours, "battery",
		"remaining battery charge in mAh (batteries that report charge_now)"),
	hostGauge("battery-energy-left", UnitWattHours, "battery",
		"remaining battery energy in Wh (batteries that report energy_now)"),
	hostGauge("temp-cpu", UnitCelsius, "temp", "CPU temperature °C (hottest CPU sensor: coretemp/k10temp/cpu_thermal…)"),
	hostGauge("temp-max", UnitCelsius, "temp", "hottest temperature °C across all detected sensors"),
}

func hostGauge(id string, unit Unit, collector, desc string) Descriptor {
	return Descriptor{
		ID: model.MetricID(id), Unit: unit, Scope: ScopeHost, Kind: KindGauge,
		Cost: Cost1FD, Collector: collector, Aggregation: AggMax, Description: desc,
	}
}

// TempSensorDescriptor describes one temperature sensor discovered at runtime
// (hwmon/thermal). Sensor sets differ per machine, so these are registered by the
// app at startup rather than listed statically.
func TempSensorDescriptor(id model.MetricID, desc string) Descriptor {
	d := hostGauge(string(id), UnitCelsius, "temp", desc)
	return d
}
