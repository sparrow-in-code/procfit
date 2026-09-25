---
id: PM-0525
title: Per-process socket & network detail (no-eBPF) + richer network profile
state: TODO
phase: 5
depends: ["PM-0501"]
owner:
rfc: ["§13.2", "§13"]
---

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

- [ ] Socket inode → `/proc/net/*` join yields per-process protocol/state counts.
- [ ] TCP state breakdown + listen-vs-connected; UDP and UNIX counts.
- [ ] No root needed for own processes; restricted rows → unavailable, not zero.
- [ ] `network` profile shows the socket detail; arch-neutral parsing.

## Tests required

- unit: `/proc/net/{tcp,udp,unix}` parser over fixtures (states, v4/v6, inode
  join); the fd→inode→state correlation; missing/denied → unavailable.

## Notes

Builds on the FD collector (PM-0501). `/proc/net/tcp` state is the hex `st`
field; map to names. The inode set per process comes from the same fd scan the
FD collector already does — share that read to avoid a second scan.
