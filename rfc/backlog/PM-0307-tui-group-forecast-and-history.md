---
id: PM-0307
title: TUI group-control §19.3 forecast + control-history surfacing
state: TODO
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

- [ ] Group control shows capture/change values + protected/skipped counts +
      a privilege/restore forecast before applying.
- [ ] The dry-run per-pid list is viewable before confirming a group action.
- [ ] Detail overlay shows recent control history for the selection when the
      audit log is enabled.

## Tests required

- unit: forecast/preview content assembly; history rendering (headless Model,
  mocked controller + recorder).

## Notes

Presentation only — reuse the Phase-2 controllers, safeguards, and the history
recorder; add no new control semantics in the TUI.
