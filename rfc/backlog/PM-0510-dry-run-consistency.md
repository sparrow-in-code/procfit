---
id: PM-0510
title: --dry-run must be honoured by all mutating control ops
state: TODO
phase: 2
depends: ["PM-0205"]
owner:
rfc: ["§15", "§20"]
---

## Summary

`--dry-run` currently only suppresses the *nice* and *freeze* mutations (freeze
fixed in PM-0502). `set --stop`/`--continue` (and `signal`) still act even with
`--dry-run`, which is a footgun for a "preview" flag. Make dry-run uniform: no
mutating control op touches a process when dry-run is set.

## Scope

- Thread `dryRun` into `SetStop` (and audit `Signal`, `Restore`, `Manage`-with-
  control) so they report the intended change without acting.
- CLI `set`/`signal`/`restore` pass `--dry-run` through; results render as
  "dry-run: would …".
- Audit: no history event recorded on a dry-run.

## Out of scope

- Nice/freeze dry-run (already implemented).

## Acceptance criteria

- [ ] `set --stop --dry-run` (and `--continue`) never sends a signal; reports the
      would-be outcome per pid.
- [ ] `signal --dry-run` never delivers; `restore --dry-run` never mutates.
- [ ] No audit events on dry-run.
- [ ] A table test asserts each mutating op is a no-op under dry-run (fake
      controller records zero mutations).

## Notes

Consistency + safety only; no new control semantics. Pairs with the freeze
blast-radius guard (PM-0502) — "preview" must always be safe.
