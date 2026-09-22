---
id: PM-0514
title: Pressure (PSI), R/D thread counts, and full delay accounting
state: TODO
phase: 5
depends: ["PM-0513"]
owner:
rfc: ["§13"]
---

## Summary

Follow-ups to the load/memory work (PM-0512/0513) that need scope-model or
privileged plumbing: system/cgroup pressure (PSI), per-process runnable/blocked
thread counts, and full per-task delay accounting.

## Scope

- **PSI** — parse `/proc/pressure/{cpu,io,memory}` (host) and per-cgroup
  `cpu.pressure`/`io.pressure`/`memory.pressure` (some/full avg10/60/300). This is
  the best single "why is load high" readout, but it is host/cgroup-scoped, so it
  needs a scope beyond process/thread (a host row or cgroup-group metric) — design
  that first (shared with the deferred cgroup-slab idea).
- **Per-process R/D thread counts** — `threads-running` (R) and
  `threads-uninterruptible` (D), each process's direct contribution to load, by
  counting `task/*/stat` states (cost-1 collector).
- **Full delay accounting** (taskstats/netlink, needs CAP_NET_ADMIN): CPU-runqueue,
  block-I/O, swap-in, and memory-reclaim/thrash delays per task — the highest-
  fidelity latency attribution, superseding the schedstat/stat proxies.

## Out of scope

- Rewriting the schedstat/stat-based `runq-delay`/`blkio-delay` (they stay as the
  no-privilege proxies).

## Acceptance criteria

- [ ] PSI cpu/io/memory pressure surfaced host-wide and per cgroup.
- [ ] threads-running / threads-uninterruptible per process; unavailable, not zero,
      when task enumeration is denied.
- [ ] Optional taskstats collector provides CPU/blkio/swap/reclaim delays, gated on
      capability, additive to the core.

## Tests required

- unit: PSI parser + scope aggregation; thread-state counting (fixture); taskstats
  decode behind an interface with a fake.

## Notes

Deferred from PM-0512/0513 to keep those low-overhead and no-privilege. PSI shares
the host/cgroup-scope design question with the cgroup-slab proposal.
