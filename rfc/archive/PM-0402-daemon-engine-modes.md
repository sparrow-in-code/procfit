---
id: PM-0402
title: Daemon engine (shared collector) + auto/require/never modes + client facade
state: DONE
phase: 4
depends: ["PM-0401"]
owner:
rfc: ["§17.1", "§17.2", "§16.4", "§23"]
---

## Summary

The per-user daemon that owns runtime state and runs one shared collector engine
for all clients, plus the client facade selecting embedded vs daemon transport.

## Scope

- `procfit daemon run` engine lifecycle (§17.1): continuous collection, authoritative
  single runtime-state writer (§16.4), event-driven notifications.
- Modes (§17.2): `auto` (connect if socket exists else embedded), `require`
  (fail without daemon), `never` (embedded). `--standalone` forces embedded.
- Client facade: same interfaces for embedded and daemon paths; two clients ⇒
  one shared collection schedule.
- Daemon diagnostics (§23): scan duration/entity count, per-collector stats,
  query/render duration, state generation, connected clients + protocol
  versions.

## Out of scope

- Config policy reconciliation (PM-0403); systemd unit + reload (PM-0404).

## Acceptance criteria

- [ ] Two clients connected to one daemon trigger one shared collection, not two
      (§22, §27.Phase4 exit).
- [ ] Mode selection matches §17.2 semantics exactly.
- [ ] Daemon is the sole runtime-state writer while running; clients use the
      socket unless `--standalone` (§16.4).
- [ ] Health/stats exposed per §23.

## Tests required

- unit: mode-selection matrix; single-writer enforcement; shared-schedule
  accounting (fake clients).
- integration (gated): two real clients + one daemon share sampling (§26.3).

## Notes

Run under `-race`. The daemon reuses the exact Phase 1/2 engine + controllers; it
adds lifecycle and sharing, not new observation/control semantics.

## Status: DONE (2026-09-12)

Implemented in the autonomous build: versioned newline-JSON AF_UNIX protocol with SO_PEERCRED same-UID auth and bounded frames; shared-collection engine (one sampler feeds all clients); client + `daemon run`/`daemon status`; continuous config-policy reconciliation (report/reapply/adopt/ignore) with capture-once originals; SIGHUP reload keeping prior policies on error; and `service install/uninstall` writing (never enabling) a user systemd unit. Tested end-to-end over a real socket.
