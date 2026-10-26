---
id: PM-0528
title: Host readouts — load average, battery state, temperatures
state: DONE
phase: 5
depends: ["PM-0506", "PM-0523"]
owner:
rfc: ["§13", "§24"]
---

## Summary

Expose whole-machine readouts as host-scoped metrics (system section / JSON
`host` / Prometheus): load averages, battery level / time left / remaining
charge-or-energy, and temperatures (curated + every detected sensor).

## Status: DONE (2026-10-02)

- **Load** (`procfs/loadavg.go`): `load-1/5/15` from `/proc/loadavg`.
- **Battery** (`procfs/battery.go`): combines all `type=Battery` supplies.
  `battery-pct` (charge or energy now/full, else mean `capacity`),
  `battery-time-left` (to empty when Discharging, to full when Charging;
  charge/current or energy/power; unavailable when full/idle),
  `battery-charge-left` (mAh) and `battery-energy-left` (Wh) — whichever the
  driver reports; a field missing on any battery invalidates the combined value
  (never a partial sum). Draw stays `power-system`.
- **Temperatures** (`procfs/temp.go`): hwmon `temp*_input` (fallback: thermal
  zones). `temp-cpu` = hottest CPU-chip sensor (coretemp/k10temp/zenpower/
  cpu_thermal/x86_pkg_temp/…), `temp-max` = hottest overall, plus one metric per
  sensor `temp-<chip>[-<label>]` (slugged, duplicates suffixed `-2`…). Sensor
  descriptors are registered at assembly time (`registerTempSensors`), so they
  appear in the catalog, `--columns`, `metrics --check`, JSON and Prometheus.
- New units: `celsius`, `mAh`, `Wh` (formatting in `render/format.go`).
- Profiles: new `system` (whole-machine dashboard); `sysload`/`pressure` gain
  `load-*`; `battery` gains `battery-pct`, `battery-time-left`, `temp-cpu`.

## Update: battery-state

`battery-state` (host, `UnitEnum`): charging/discharging/full/not-charging/
missing/unknown, `mixed` when batteries disagree (a discharging or charging one
wins over idle/full ones). Metric values stay numeric — the state is a stable
code (`model.BatteryState`, append-only mapping) rendered as its label in text
output via `model.EnumLabel` (a per-enum-metric label table, so adding an enum
metric is one entry); JSON adds `label` next to the code; Prometheus exports the
code. `present=0` bays report `missing`; no battery hardware → unavailable. Added
to the `battery` and `system` profiles. Tests: `TestBatteryCollector_State`,
`TestBatteryStateLabelsAndStatus`, `TestFormatMetric_EnumLabel`.

## Tests

Fixture sysfs/procfs trees under `t.TempDir()` only: `TestLoadAvgCollector`,
`TestBatteryCollector_*` (charge-style discharging, energy-style charging, two
batteries + full + capacity fallback + none), `TestTempCollector_*` (hwmon
labels/dupes/cpu-vs-max, thermal fallback, none), `TestSlugAndTempID`,
`app.TestRegisterTempSensors`, unit format cases.

## Notes

Not smoke-tested on a live host (per owner instruction to stay within the repo);
verified via fixtures + `make check`. All cost-1, so the default light view does
no extra host reads.
