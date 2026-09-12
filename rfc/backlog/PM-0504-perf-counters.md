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

## Status: partial (2026-09-12)

Capability-gating done: metric descriptors registered (render unavailable, never zero), profiles/columns resolve, and `capabilities` reports the backend as unsupported/permission_denied. The actual eBPF/perf backend is deferred — it needs cgo + kernel headers + CAP_BPF/CAP_PERFMON (or lowered perf_event_paranoid) and cannot be built/tested in this rootless environment. This matches RFC intent that these backends are optional and must never be mandatory.

## Status: capability layer DONE; full backend BLOCKED (2026-09-12)

Done: metric descriptors registered (cost 2/3), profiles reference them, `capabilities` reports BPF/perf availability + remediation, and absent backends yield unavailable (never zero) — satisfying the RFC's 'optional, degrade gracefully' contract. Blocked: the real eBPF/perf collectors need a BPF toolchain + CO-RE, elevated privileges (CAP_BPF/CAP_PERFMON or relaxed perf_event_paranoid), and a suitable kernel — none available or verifiable in this rootless dev/CI environment. Deferred rather than shipping an untestable loader (see QUESTIONS.md item D). The collector seam (MetricCollector port) is ready to plug a backend in.
