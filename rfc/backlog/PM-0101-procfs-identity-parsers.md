---
id: PM-0101
title: Safe procfs reader, PID/starttime identity, and parsers
state: TODO
phase: 1
depends: ["PM-0004"]
owner:
rfc: ["§6.3", "§11", "§14.1", "§21.1", "§26.1", "§26.2"]
---

## Summary

The foundation of all observation: an abstracted procfs reader rooted at a
configurable path, robust parsers for `stat`/`status`/`io`/etc., and the
canonical process identity (`boot_id + pidns_inode + pid + starttime`).

## Scope

- `internal/procfs` reader behind an interface rooted at a configurable path
  (real `/proc` in prod, fixture tree in tests) — same interface PM-0004 faked.
- `/proc/<pid>/stat` parser that correctly handles comm containing spaces and
  parentheses (locate final `)`; never naive whitespace split) — RFC §14.1.
- Parsers for `status`, `io`, `statm`, `cmdline`, `stat` field 22 (starttime),
  namespace inodes from `/proc/<pid>/ns/*`, cgroup path.
- `boot_id` from `/proc/sys/kernel/random/boot_id`.
- `ProcessInstanceID` / `ThreadInstanceID` construction and equality (§6.3).
- Vanish handling: `ENOENT`/`ESRCH` is a normal outcome, aggregated into stats,
  not per-occurrence warnings (§21.1).

## Out of scope

- Rates/metrics (PM-0103); resolvers/labels (PM-0104); scheduling (PM-0102).

## Acceptance criteria

- [ ] `stat` parser handles `comm` like `((weird )name)` and unusual bytes.
- [ ] Identity compares pid AND starttime; a reused pid with a different
      starttime is a different instance (§6.3).
- [ ] Process disappearing mid-read yields `Vanished` availability, not error
      spam or zeroes.
- [ ] `cmdline` distinguishes empty from unreadable (§11.1).
- [ ] All reads go through the rooted interface; no hardcoded `/proc`.

## Tests required

- unit: parser edge cases; identity equality/inequality; boot-id read.
- property/fuzz: stat/status/io parsers never panic on arbitrary bytes (§26.2).
- integration (later, gated): real `/proc` smoke where available.

## Notes

**RFC §31.5:** never key state by numeric PID alone. This ticket makes correct
identity available to everything downstream; controllers (Phase 2) revalidate it
before acting.
