---
id: PM-9005
title: "RFC amendment: remote/multi-host monitoring over SSH-tunneled daemon"
state: TODO
phase: 4
depends: ["PM-0402"]
owner:
rfc: ["§17"]
---

## Summary

Amend the RFC to allow monitoring/troubleshooting several hosts from one client
(e.g. a laptop watching ~5 home devices) in the same TUI, by connecting to a
`procfit daemon` on each host. The daemon already speaks JSON over a plain
`net.Conn`, and the TUI depends only on `RefreshFunc`, so a remote host is a
daemon client wired into the TUI — this amendment defines the transport and the
control-authorization model.

## Proposed amendment

- **Transport: SSH, not an open port.** Add a `--stdio` transport to `daemon`
  (speak the protocol on stdin/stdout) and a client that runs
  `ssh user@host procfit daemon --stdio`. Reuses SSH auth + encryption, opens no
  network ports, and the remote daemon keeps its local `peercred` uid check.
  A raw TCP/TLS listener MAY be added later but is off by default (a monitoring +
  control daemon on a port is a footgun).
- **Client routing:** `procfit --host user@host tui|ps|stat` dials the remote;
  requires the `daemon.use` client-routing seam (see notes) to also back the
  local embedded/daemon choice.
- **Multi-host UX:** the TUI gains a host list / switcher (each host = one client);
  a later step MAY aggregate across hosts (host as a group dimension).
- **Control over remote:** renice/stop/signal on a remote host must revalidate
  identity remotely and remain never-root/degrade-by-capability; destructive
  actions keep the confirmation gate. Read-only remote monitoring is the default;
  remote control is explicit.

## Out of scope

- Central server / fan-in aggregation service; push metrics (that is the
  Prometheus exporter, PM-9006).
- Auto-discovery of hosts.

## Acceptance criteria

- [ ] `daemon --stdio` speaks the full protocol over stdin/stdout.
- [ ] `--host user@host` backs the TUI/ps/stat via an SSH-spawned remote daemon;
      no ports opened.
- [ ] TUI switches between ≥2 hosts; each shows its own live metrics.
- [ ] Remote control revalidates identity and honours safeguards + confirmation.

## Tests required

- unit: stdio transport round-trip (protocol over an in-memory pipe); remote
  RefreshFunc against a fake conn; multi-host model switching (headless).
- integration (gated): two local daemons over stdio pipes.

## Notes

Prereq: wire `daemon.use` (auto|require|never) client routing — today ps/stat/set
always embed; only `daemon status` dials. Remote is that seam pointed at an
SSH-spawned conn. Present as an RFC amendment to §17 (which is currently
per-user, local, Unix-socket only) before implementing.
