---
id: PM-0504
title: Optional perf/hardware counter collector (cost level 3)
state: TODO
phase: 5
depends: ["PM-0102"]
owner:
rfc: ["§13.4", "§20.2"]
---

## Summary

Opt-in hardware performance counters via perf_event, with slower default
interval and multiplexing/scaling metadata.

## Scope

- Metrics §13.4: cycles, instructions, ipc, cache-misses,
  context-switches-perf.
- May require `perf_event_paranoid` changes/capabilities (§13.4, §20.2); slower
  default interval; report multiplexing/scaling metadata on every sample.
- Capability probe reports perf access + remediation (§14.3).

## Out of scope

- eBPF collectors (PM-0503).

## Acceptance criteria

- [ ] Opt-in only; unavailable ⇒ values unavailable with reason, never zero.
- [ ] Multiplexing/scaling metadata present on samples (§13.4).
- [ ] Slower default interval than light collectors; disabled adds no work.

## Tests required

- unit: scaling-metadata propagation; availability on paranoid/denied; interval
  scheduling.

## Notes

Individually shippable (§27.Phase5). Never require root by default (§20.1).
