---
id: PM-0401
title: AF_UNIX IPC protocol (versioned) with peer-credential auth
state: DONE
phase: 4
depends: ["PM-0201", "PM-0106"]
owner:
rfc: ["§17.3", "§20.3"]
---

## Summary

The versioned client/daemon wire protocol over a user-private Unix socket,
authenticated by socket ownership and peer credentials.

## Scope

- `AF_UNIX` transport; length-prefixed frames or size-bounded NDJSON (§17.3).
- Messages (§17.3): hello/version/capabilities, subscribe snapshot stream, run
  query, list/inspect managed, dry-run + execute control plan, restore/unmanage,
  reload config, health/collector stats.
- Auth via `SO_PEERCRED` + socket ownership; reject different UID by default;
  bound request/response sizes (§17.3).
- Protocol version negotiation; forward/backward-compat policy documented.

## Out of scope

- Daemon engine lifecycle (PM-0402); reconciliation (PM-0403).

## Acceptance criteria

- [ ] Protocol is versioned; mismatched versions negotiate or fail cleanly.
- [ ] Connections from a different UID are rejected by default.
- [ ] Oversized requests/responses are rejected, not buffered unbounded.
- [ ] Exit code 10 on protocol/communication errors (§8.7).

## Tests required

- unit: framing/encode-decode; peer-cred rejection (fake creds); size limits;
  version negotiation.
- property/fuzz: message decoder rejects malformed/oversized input safely.

## Notes

The client uses the same `Engine`/`Controller` interfaces (§25) whether embedded
or over IPC — this ticket implements the remote transport behind those
interfaces (dependency inversion).

## Status: DONE (2026-09-12)

Implemented in the autonomous build: versioned newline-JSON AF_UNIX protocol with SO_PEERCRED same-UID auth and bounded frames; shared-collection engine (one sampler feeds all clients); client + `daemon run`/`daemon status`; continuous config-policy reconciliation (report/reapply/adopt/ignore) with capture-once originals; SIGHUP reload keeping prior policies on error; and `service install/uninstall` writing (never enabling) a user systemd unit. Tested end-to-end over a real socket.
