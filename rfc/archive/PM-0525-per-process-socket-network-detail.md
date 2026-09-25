---
id: PM-0525
title: Per-process socket & network detail (no-eBPF) + richer network profile
state: DONE
phase: 5
depends: ["PM-0501"]
owner:
rfc: ["§13.2", "§13"]
---

## Status: DONE (2026-09-23)

`procfs.SocketCollector` (`internal/procfs/socket.go`) reads the caller's
`/proc/net/{tcp,tcp6,udp,udp6,unix}` once into an inode→{proto,state} map
(`mergeNetTable`), then classifies each process's socket fds
(`/proc/PID/fd → socket:[inode]`, `socketInode`) against it — no eBPF/root.
Metrics (`internal/metrics/builtin_socket.go`, ScopeProcess/Cost1FD/AggSum):
`sock-tcp`/`sock-udp`/`sock-unix` and TCP-state `sock-listen`/`sock-estab`/
`sock-timewait`/`sock-closewait`. Bounded-concurrency per-process scan (shares the
`maxFDScans` semaphore). Denied fd dir → unavailable, never zero. The `network`
profile now leads with these counts (sorted by `sock-tcp`) and keeps the eBPF
`net-*-bps/pps` as the throughput layer. Validated live (chrome 41 TCP, idea 15
TCP/4 listening). **Limitation (noted):** reads the collector's own netns view, so
sockets held by processes in another netns aren't classified (not wrong, just not
counted); byte/packet throughput stays the eBPF path. Commit: (local).

## Summary

Give each process a real network picture without eBPF or root: how many TCP/UDP/
UNIX sockets it holds, how many are listening vs connected, and the TCP state
breakdown (ESTABLISHED / TIME_WAIT / CLOSE_WAIT / LISTEN / …). The FD collector
already classifies `socket:` fds into `fd-sockets`; this correlates those socket
inodes with `/proc/net/*` to classify them. Per-process byte/packet throughput
stays the eBPF path (the existing `net-*-bps/pps`), since `/proc/net/*` is
socket-scoped, not per-process traffic.

## Scope

- Extend the socket path: for each process collect its socket inodes from
  `/proc/PID/fd/* -> socket:[inode]`, then join against parsed
  `/proc/net/{tcp,tcp6,udp,udp6,unix}` (inode column) for protocol + state +
  local/remote addr.
- New metrics (ScopeProcess, Cost1FD, AggSum): `sock-tcp`, `sock-udp`,
  `sock-unix`, `sock-listen`, `sock-estab`, `sock-timewait`, `sock-closewait`
  (and a small curated state set). Server = LISTEN; client = connected with a
  remote peer.
- Enrich the `network` profile to lead with these no-root counts, keeping the
  eBPF `net-*-bps/pps` as the (privileged) throughput layer.
- Degrade honestly: another user's `/proc/net` rows / fd links may be
  unreadable → unavailable, never zero.

## Out of scope

- Per-process bytes/packets (needs eBPF or pcap; the eBPF `net-*` metrics cover
  it when a backend is present).
- Per-connection detail rows / a connections view (possible follow-up).

## Acceptance criteria

- [x] Socket inode → `/proc/net/*` join yields per-process protocol/state counts.
- [x] TCP state breakdown + listen-vs-connected; UDP and UNIX counts.
- [x] No root needed for own processes; restricted rows → unavailable, not zero.
- [x] `network` profile shows the socket detail; arch-neutral parsing.

## Tests required

- unit: `/proc/net/{tcp,udp,unix}` parser over fixtures (states, v4/v6, inode
  join); the fd→inode→state correlation; missing/denied → unavailable.

## Notes

Builds on the FD collector (PM-0501). `/proc/net/tcp` state is the hex `st`
field; map to names. The inode set per process comes from the same fd scan the
FD collector already does — share that read to avoid a second scan.
