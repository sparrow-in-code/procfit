---
id: PM-0526
title: Per-process network throughput backend (SOCK_DIAG / eBPF) for net-*
state: TODO
phase: 5
depends: ["PM-0525"]
owner:
rfc: ["§13", "§13.3"]
---

## Summary

Actually populate the declared per-process network throughput metrics
(`net-rx-bps`, `net-tx-bps`, `net-rx-pps`, `net-tx-pps`). They exist in the
registry (Collector "ebpf") but nothing emits them today — only the `wakeups`
eBPF program is built — so they always render `-`. `/proc` has no per-process
traffic counters, so this needs one of two backends.

## Scope

- **SOCK_DIAG / inet_diag (no root, preferred first):** query TCP sockets via
  `NETLINK_INET_DIAG` (SOCK_DIAG) for `tcp_info` (bytes_sent/bytes_received,
  segs_in/segs_out), map each socket back to its owning process by inode (reuse
  the PM-0525 fd→inode set), and delta per socket across samples → per-process
  `net-tx-bps`/`net-rx-bps` (+ pps from segments). No root for own sockets;
  degrade to unavailable otherwise. TCP only (UDP has no equivalent counters).
- **eBPF (richer, privileged) alternative:** a dedicated BPF program (sock/skb or
  cgroup) attributing bytes/packets per pid, behind the same metric ids — the
  full picture incl. UDP; register as the preferred source when a BPF backend is
  active (mirrors the wakeups eBPF-vs-procfs fallback).
- Handle socket churn / counter reset across samples (per-socket identity), and
  re-add `net-*` to the `network` profile view once a backend is present.

## Out of scope

- Per-connection throughput rows / a connections view.
- Historical/rolled-up bandwidth.

## Acceptance criteria

- [ ] `net-tx-bps`/`net-rx-bps` populated from SOCK_DIAG for own TCP sockets
      (no root); unavailable (not zero) when the source can't read.
- [ ] Rate derived as a per-socket delta over the sample interval; socket churn
      and counter resets handled.
- [ ] Optional eBPF backend behind the same ids, preferred when active.
- [ ] `network` profile shows throughput again once a backend exists.

## Tests required

- unit: tcp_info parse over fixture bytes; inode→pid attribution; rate/delta math
  with socket churn; SOCK_DIAG round-trip behind an interface with a fake.

## Notes

Spun off from the PM-0525 socket work (which shipped the no-root socket *counts*
and intentionally left throughput to a backend). SOCK_DIAG is the no-root win for
TCP; eBPF is the complete (privileged) picture.
