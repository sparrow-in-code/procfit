---
id: PM-0507
title: CPU idle (C-state) residency collector
state: TODO
phase: 5
depends: ["PM-0501"]
owner:
rfc: ["§13", "§24"]
---

## Summary

Report how much time CPUs spend in each idle (C-)state. Deep-idle residency is
the direct correlate of battery life; *low residency + high wakeups* is the drain
signature that neither cpu% nor wakeups alone reveal. This is host/per-CPU
context that frames the per-process wakeup metrics.

## Scope

- Read `/sys/devices/system/cpu/cpu*/cpuidle/state*/{name,time,usage,disable}` and
  compute per-state residency (% of interval) and entry rate (usage/s), delta
  over the sample interval.
- System-level aggregate metrics: `cstate-deep-residency` (% in the deepest
  states), plus a per-state breakdown available via a wide/verbose view.
- Degrade to unavailable when cpuidle is absent (e.g. `intel_idle`/`acpi_idle`
  disabled, or a minimal VM).

## Out of scope

- Per-process attribution (C-states are a CPU property, not a process one).
- MSR-based package C-states (turbostat-style) — a possible follow-up; the
  cpuidle sysfs path is the portable baseline.

## Acceptance criteria

- [ ] Per-CPU and aggregate residency computed from cpuidle sysfs, delta-based.
- [ ] Works unchanged on any arch with the cpuidle framework (x86 and ARM);
      no vendor/driver name hardcoded (state names are read, not assumed).
- [ ] Absent cpuidle → unavailable, never fabricated zeros.

## Tests required

- unit: cpuidle parser over a fixture tree (multiple CPUs/states); residency and
  rate math over an interval; missing-framework → unavailable.

## Notes

The cpuidle framework is arch-agnostic, so this is vendor- and arch-neutral by
construction — Intel, AMD, and ARM cpuidle all expose the same sysfs shape.
