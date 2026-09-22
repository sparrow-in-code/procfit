---
id: PM-0510
title: --dry-run must be honoured by all mutating control ops
state: DONE
phase: 2
depends: ["PM-0205"]
owner:
rfc: ["§15", "§20"]
---

## Status: DONE (2026-09-22)

`dryRun` threaded through `Manager.SetStop`/`Signal`/`Restore`/`RestoreNice`
(`internal/control/manager_ops.go`): each revalidates identity + safeguards, then
short-circuits with `StatusUnchanged` and a `dry-run: would …` message before any
`SendSignal`/`SetNice`/`audit`, and never records target intent (`DesiredStop`) or
clears it (`Restore`). CLI `restore`/`signal` gained `--dry-run` (`set` already had
it); a destructive `signal --dry-run` skips the `--yes` gate since nothing is sent.
Daemon protocol carries `dry_run` (`ControlReq.DryRun`) and skips the state save on
a preview. Tests: `control.TestControl_DryRunNeverMutates` (table: stop/signal/
restore assert zero signals, unchanged nice, zero audit events, intent preserved)
and `app.TestControl_SetStopContinueAndDryRun` extended for CLI stop/signal/restore.
Commit: (local).

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

- [x] `set --stop --dry-run` (and `--continue`) never sends a signal; reports the
      would-be outcome per pid.
- [x] `signal --dry-run` never delivers; `restore --dry-run` never mutates.
- [x] No audit events on dry-run.
- [x] A table test asserts each mutating op is a no-op under dry-run (fake
      controller records zero mutations).

## Notes

Consistency + safety only; no new control semantics. Pairs with the freeze
blast-radius guard (PM-0502) — "preview" must always be safe.
