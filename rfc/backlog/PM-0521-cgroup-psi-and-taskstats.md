---
id: PM-0521
title: Per-cgroup PSI and taskstats/netlink delay accounting
state: TODO
phase: 5
depends: ["PM-0514"]
owner:
rfc: ["§13"]
---

## Summary

The two pieces carved out of PM-0514 because they need plumbing beyond the
process/host scope model that PM-0514 delivered: per-cgroup pressure and the
privileged, highest-fidelity per-task delay accounting.

## Scope

- **Per-cgroup PSI** — parse `cpu.pressure`/`io.pressure`/`memory.pressure` under
  each cgroup and expose it as a **cgroup-group** metric. This needs a scope
  beyond process/host (a per-group readout keyed by cgroup), the same seam the
  cgroup-slab idea wants — design that group-scope first, then reuse it.
- **Full delay accounting (taskstats/netlink, CAP_NET_ADMIN):** CPU-runqueue,
  block-I/O, swap-in, and memory-reclaim/thrash delays per task — the highest-
  fidelity latency attribution, superseding the schedstat/stat proxies. Put it
  behind a `ports.DelayAccounting` interface with a fake for tests; degrade to
  unavailable (never zero) when the capability is absent.

## Out of scope

- Replacing the no-privilege `runq-delay`/`blkio-delay` proxies (they stay as the
  default, unprivileged signals; taskstats is additive when permitted).
- Host PSI and per-process R/D thread counts (done in PM-0514).

## Acceptance criteria

- [ ] Per-cgroup PSI surfaced via a cgroup group-scope metric; absent → unavailable.
- [ ] taskstats collector behind an interface, capability-gated, additive; a fake
      drives the decode in tests.
- [ ] No new default cost when the capability is off.

## Tests required

- unit: cgroup PSI parser + group-scope aggregation; taskstats decode behind the
  interface with a fake netlink source.

## Notes

Split from PM-0514 (host PSI + R/D counts shipped there). The cgroup group-scope
model is the shared blocker; land it once and both cgroup PSI and any future
cgroup-slab/cgroup-power attribution reuse it.
