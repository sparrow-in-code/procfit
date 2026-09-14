---
id: PM-0308
title: Control-history reader + history in the TUI detail overlay
state: DONE
phase: 5
depends: ["PM-0307", "PM-0505"]
owner:
rfc: ["§16.1", "§19.2"]
---

## Summary

Surface control history (apply/restore actions) for a process/target. Today the
audit `FileRecorder` is write-only (`Record`); nothing reads it back.

## Scope

- A reader for the audit log: load recent `history.Event`s, filter by pid/target,
  bounded/tailed for cheap access.
- Show recent control history for the selected row in the TUI detail overlay
  (`i`), when the audit log is enabled (`[state].history`).
- CLI parity: a `history` sub-command (or `managed --history`) to print recent
  events, so the data is available headless too.

## Out of scope

- Recording semantics (PM-0505) and the TUI control actions/overlay shell
  (PM-0303/PM-0306/PM-0307), which already exist.

## Acceptance criteria

- [x] Audit events can be read back (bounded) and filtered by pid/target
      (`history.ReadRecent`).
- [x] Detail overlay lists recent apply/restore actions for the selection when
      history is enabled; shows a "(none — enable [state].history)" hint otherwise.
- [x] A headless path prints the same events (`procfit history [--pid N]`).

## Tests required

- unit: reader parses/tails the log; overlay renders events; empty/disabled
  states (headless Model + temp log).

## Notes

Read-only over the existing recorder format; no change to control semantics.

## Status: DONE (2026-09-13)

Added `history.ReadRecent` (bounded, filterable NDJSON reader). The shared control
assembly now enables the `FileRecorder` when `[state].history` is set (so CLI
`set`/`restore` and TUI control record, not just the daemon), and
`loadEffectiveConfig` honours `$PROCFIT_CONFIG`. Surfaced via the `procfit
history [--pid N]` command and the TUI detail overlay (`i`). Verified end to end:
a renice with history enabled appears in both.
