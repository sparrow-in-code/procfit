---
id: PM-0201
title: Runtime state store — atomic write, locking, boot scoping, safe perms
state: TODO
phase: 2
depends: ["PM-0101"]
owner:
rfc: ["§16", "§16.2", "§16.3", "§16.4", "§16.5", "§26.1"]
---

## Summary

Durable-enough, private runtime state for bindings, captured originals, and
desired/observed values — atomically written, boot-scoped, single-writer.

## Scope

- Three state classes + XDG locations (§16.1); runtime dir resolution with safe
  `/run/user/<uid>` fallback and private-temp last resort (never shared
  `/tmp/pm`).
- Layout (§16.2): `state.json`, `state.lock`, `pm.sock` placeholder,
  `events.ndjson`; dir `0700`, files `0600`; refuse foreign-UID/unsafe-symlink;
  `openat2`/safe-open where practical.
- Versioned JSON state schema (§16.3) with atomic write: temp → fsync → rename →
  dir fsync; keep `.bak`; quarantine corrupt files instead of overwriting.
- Advisory exclusive lock for mutations, shared/snapshot for reads (§16.4).
- Boot handling (§16.5): on boot-id mismatch discard PID/TID bindings + captured
  originals; retain config policies via reload; never rebind by PID/inode alone.
- `--state-dir` override (§8.1).

## Out of scope

- Managed target semantics (PM-0202); daemon single-writer (Phase 4).

## Acceptance criteria

- [ ] Directory/file perms enforced; foreign-owned or symlinked paths refused.
- [ ] Atomic write survives simulated crash mid-write (no partial state loaded).
- [ ] Corrupt state is quarantined, not silently overwritten (§16.3).
- [ ] Boot-id mismatch discards concrete bindings but keeps logical policy.
- [ ] Never writes to a shared predictable path (§16.1).

## Tests required

- unit: perm/ownership checks; atomic write + recovery; lock exclusivity;
  boot-id transition; quarantine path.
- property/fuzz: state decoder rejects malformed/oversized input safely (§26.2).

## Notes

This underpins safe control. Single-responsibility: persistence only — no
control logic here.
