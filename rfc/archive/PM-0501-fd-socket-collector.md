---
id: PM-0501
title: FD/socket classification collector (cost level 1)
state: DONE
phase: 5
depends: ["PM-0102"]
owner:
rfc: ["§13.2", "§22"]
---

## Summary

Periodic (default 5s) collector classifying open file descriptors and sockets by
type, with bounded concurrency and sampled quality.

## Scope

- Metrics §13.2: fd-total/files/sockets/pipes/anon/eventfd/epoll/timerfd/
  signalfd/inotify; socket-tcp/udp/unix/netlink.
- Scan `/proc/PID/fd` symlinks; correlate socket inodes with procfs/netlink
  socket tables (§13.2). Races/permission failures expected ⇒ sampled quality.
- Bounded concurrency: never more than 32 proc-dir scans in flight (§22).

## Out of scope

- eBPF/perf collectors (PM-0503/PM-0504).

## Acceptance criteria

- [ ] Counts carry `sampled` quality; permission/vanish ⇒ unavailable, not zero.
- [ ] Concurrency cap enforced; no I/O storm on high PID counts (§22).
- [ ] Disabled by default cost-1 collector adds no work to the light profile.

## Tests required

- unit: fd classification from fixtures; socket inode correlation; race/denied
  handling; concurrency bound.
- perf: cost-1 scan against synthetic fixtures within budget.

## Notes

Individually shippable; must not increase default light-profile work when
disabled (§27.Phase5).

## Status: DONE (2026-09-12)

MetricCollector port + procfs FDCollector: classifies /proc/PID/fd links (files/sockets/pipes/anon + eventfd/epoll/timerfd/signalfd/inotify) with Sampled quality and bounded concurrency (≤32); enrichment runs only when fd metrics are requested (disabled = no work). socket-tcp/udp/unix/netlink subtypes (netlink correlation) deferred. Also registered cost-2 (event/eBPF) and cost-3 (perf) descriptors so profiles/columns resolve while rendering unavailable until a backend exists.
