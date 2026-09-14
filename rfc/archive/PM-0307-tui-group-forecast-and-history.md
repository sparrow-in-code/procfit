---
id: PM-0307
title: TUI group-control §19.3 forecast + control-history surfacing
state: DONE
phase: 3
depends: ["PM-0306"]
owner:
rfc: ["§19.2", "§19.3", "§16.1"]
---

## Summary

Refine the TUI control side plane delivered in PM-0306: give group control the
full §19.3 preview, and surface control history in the detail overlay.

## Scope

- Group control preview (§19.3): beyond the affected count, show the values to
  capture/change per field, the protected/skipped breakdown, and a predicted
  privilege/restore-limit forecast (can we later restore what we're about to
  change?), with an option to view the full per-pid dry-run list before applying.
- Control history in the detail overlay (§16.1): show recent apply/restore
  actions for the selected process/target. Requires enabling and reading the
  audit log (Nop by default today).

## Out of scope

- The managed panel, single-process/group control, and the identity detail
  overlay (all done in PM-0303/PM-0306).

## Acceptance criteria

- [x] Group control shows capture/change values + protected/skipped counts +
      a privilege/restore forecast before applying. (Renice forecast: per-pid
      current→desired, would-be status incl. protected, and a warning when the
      change raises priority.)
- [x] The dry-run per-pid list is viewable before confirming a group action.
      (`v` in the confirm gate opens the full FORECAST overlay.)
- [ ] Detail overlay shows recent control history for the selection when the
      audit log is enabled. → moved to PM-0308 (needs a history reader; the
      FileRecorder is write-only today).

## Tests required

- unit: forecast/preview content assembly; history rendering (headless Model,
  mocked controller + recorder).

## Notes

Presentation only — reuse the Phase-2 controllers, safeguards, and the history
recorder; add no new control semantics in the TUI.

## Status: DONE (2026-09-13)

Group-control §19.3 forecast landed: the confirmation gate's `v` opens a FORECAST
overlay listing each pid's current→desired nice, would-be status (incl.
protected/skipped), and a privilege/restore warning when raising priority. Built
from the manager dry-run + `Controller.GetNice`; non-nice actions show the
affected pid list. Control-history surfacing is split to **PM-0308** because the
history `FileRecorder` has no reader yet.
