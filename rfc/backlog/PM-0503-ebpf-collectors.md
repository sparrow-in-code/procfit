---
id: PM-0503
title: Optional eBPF collectors (wakeups, network, syscall rates)
state: TODO
phase: 5
depends: ["PM-0102"]
owner:
rfc: ["§13.3", "§30.5", "§30.6"]
---

## Summary

Cost-level-2 event-tracing collectors (usually eBPF) for metrics not obtainable
from `/proc` scanning, each with a written attribution definition.

## Scope

- Metrics §13.3: wakeups, timer/futex/poll-wakes, scheduler-migrations,
  runqueue-delay, open/close/read/write/fsync/fdatasync rates,
  connect/accept rate, net-rx/tx bps/pps.
- **Each metric needs a written attribution definition** (§13.3) — e.g. wakeup
  attributed to awakened vs waking task. Do not ship an ambiguous `wakeups`.
- Per-process network bytes only available with this backend active (§13.3).
- eBPF distribution strategy decision (§30.5): embedded object / CO-RE / build
  tag — record as PM-90NN note. Must not be mandatory (§4).

## Out of scope

- perf/hardware counters (PM-0504).

## Acceptance criteria

- [ ] eBPF is optional; absence ⇒ these metrics unavailable, never zero, and the
      light profile is unaffected.
- [ ] Every shipped metric has a documented attribution semantic (§13.3).
- [ ] Capability probe reports BPF availability + required privileges (§14.3).

## Tests required

- unit: attribution logic over synthetic event streams; availability when BPF
  absent/denied.
- integration (gated, where BPF permitted): rate sanity vs spawned helpers.

## Notes

Wakeup attribution names/semantics are an open question (§30.6). Resolve via a
PM-90NN note before shipping the metric.

## Status: partial (2026-09-12)

Capability-gating done: metric descriptors registered (render unavailable, never zero), profiles/columns resolve, and `capabilities` reports the backend as unsupported/permission_denied. The actual eBPF/perf backend is deferred — it needs cgo + kernel headers + CAP_BPF/CAP_PERFMON (or lowered perf_event_paranoid) and cannot be built/tested in this rootless environment. This matches RFC intent that these backends are optional and must never be mandatory.

## Status: capability layer DONE; full backend BLOCKED (2026-09-12)

Done: metric descriptors registered (cost 2/3), profiles reference them, `capabilities` reports BPF/perf availability + remediation, and absent backends yield unavailable (never zero) — satisfying the RFC's 'optional, degrade gracefully' contract. Blocked: the real eBPF/perf collectors need a BPF toolchain + CO-RE, elevated privileges (CAP_BPF/CAP_PERFMON or relaxed perf_event_paranoid), and a suitable kernel — none available or verifiable in this rootless dev/CI environment. Deferred rather than shipping an untestable loader (see QUESTIONS.md item D). The collector seam (MetricCollector port) is ready to plug a backend in.
