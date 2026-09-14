---
id: PM-0506
title: Vendor-neutral energy/power collector (RAPL/hwmon/battery)
state: TODO
phase: 5
depends: ["PM-0501"]
owner:
rfc: ["§13", "§24"]
---

## Summary

Report actual power/energy draw (watts), not just proxies. This is the biggest
gap for battery/power work: today everything (cpu, wakeups) is a correlate. Add a
`PowerSource` port with several backends so we read whatever the host exposes
without locking to one vendor.

## Scope

- Define a `ports.PowerSource` interface (energy domains + read energy_uj/power).
  Adapters (tried in order, first that works wins; capability-gated):
  - **powercap / RAPL** — `/sys/class/powercap/*/energy_uj` (Intel `intel-rapl:*`
    AND AMD, which uses the same powercap tree on modern kernels) with domains
    package / core / uncore / dram / psys. Delta energy → watts.
  - **hwmon** — `/sys/class/hwmon/*/{power,energy}*_input` (broad vendor coverage,
    incl. some AMD `amd_energy` and platform sensors).
  - **battery (power_supply)** — `/sys/class/power_supply/BAT*/` `power_now`, or
    `voltage_now × current_now`. This is the **vendor- and arch-neutral**
    whole-system draw on any laptop (Intel/AMD/ARM), and works on discharge.
  - **perf RAPL events** (`power/energy-pkg/…`) as an alternative to powercap.
- Metrics: system/socket `power-pkg`, `power-core`, `power-dram`, `power-uncore`,
  and `power-system` (from battery). Domains that don't exist degrade to
  unavailable (never zero).
- Optional per-cgroup attribution as an *estimate* (weight socket package power by
  the cgroup's share of busy CPU time); clearly labelled as estimated.

## Out of scope

- Per-process watts modelling (powertop-style) — a later ticket if wanted.
- macOS/Windows power sources (see Notes; a future darwin adapter would use
  IOKit/SMC, not sysfs).

## Acceptance criteria

- [ ] `PowerSource` port with ≥2 backends; selection is capability-gated and the
      active backend is reported by `procfit capabilities`.
- [ ] No vendor string hardcoded in the core; Intel and AMD both work via the
      powercap/hwmon paths; battery backend works with no RAPL at all.
- [ ] Energy is read as a delta (handles counter wrap) → per-second watts; missing
      domains render unavailable.
- [ ] A `power` metric set surfaces in `--metrics power` (redefine `power` around
      real watts; keep `battery` as the per-process drain drivers — see Notes).

## Tests required

- unit: powercap/hwmon/battery parsers over fixture sysfs trees; counter-wrap
  handling; domain-missing → unavailable; backend selection order.

## Notes

Keep vendor/arch differences as *adapter* details behind the port. ARM has no
RAPL but does have `power_supply` (battery) and often hwmon, so ARM benefits via
those backends. Once real watts land, split the profiles cleanly:
`power` = actual consumption (watts/energy), `battery` = per-process drivers of
drain (wakeups, ctxsw, cpu, gpu).
