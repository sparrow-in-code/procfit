---
id: PM-0308
title: Control-history reader + history in the TUI detail overlay
state: TODO
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

- [ ] Audit events can be read back (bounded) and filtered by pid/target.
- [ ] Detail overlay lists recent apply/restore actions for the selection when
      history is enabled; shows a clear "history off" hint otherwise.
- [ ] A headless path prints the same events.

## Tests required

- unit: reader parses/tails the log; overlay renders events; empty/disabled
  states (headless Model + temp log).

## Notes

Read-only over the existing recorder format; no change to control semantics.
