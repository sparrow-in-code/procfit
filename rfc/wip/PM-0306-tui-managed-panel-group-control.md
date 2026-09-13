---
id: PM-0306
title: TUI managed panel, detail overlay, and group control preview
state: WIP
phase: 3
depends: ["PM-0303"]
owner:
rfc: ["§19.1", "§19.2", "§19.3"]
---

## Summary

Complete the TUI control side plane deferred from PM-0303 (which shipped
single-process control with per-action confirmation).

## Scope

- Managed panel (§19.1): retained + INACTIVE targets with last-seen; `Tab`
  switches between the process browser and the managed panel; `m`
  manage/unmanage.
- Detail overlay (§19.2): `i`/Enter inspect a row (identity, cgroup, control
  history).
- Group/selector control from the TUI with the full §19.3 preview: logical
  target + binding mode, exact match count, protected/skipped count, values to
  capture/change, and a predicted privilege/restore-limit forecast; user can
  view the dry-run list before applying.

## Out of scope

- Single-process control keys (done in PM-0303).
- Daemon-backed control (Phase 4).

## Acceptance criteria

- [ ] Managed panel lists retained + INACTIVE targets with last-seen; `Tab`
      switches panels.
- [x] Group/selector control shows a preview before applying. (A group row's
      control collects every descendant process; the preview shows the affected
      count, and nice's dry-run aggregates status counts incl. protected/denied.
      A richer §19.3 capture/change + privilege forecast is still open.)
- [ ] Detail overlay shows identity + control history for the selected row.
- [x] Control still reuses the Phase-2 controllers/safeguards (no duplicated
      control logic in the TUI).

## Status

**2026-09-13:** group control landed — throttling from a selected group row acts
on all member processes via the same `tui.ControlFunc`/`control.Manager` path,
with a count-aware preview + confirmation. Managed panel, detail overlay, and the
full §19.3 capture/change + privilege forecast remain open.

## Tests required

- unit: managed-panel state + Tab switching; group-preview content assembly;
  privilege forecast; detail overlay rendering (headless Model, mocked
  controller).

## Notes

The TUI remains a client of the same control planner as the CLI (§6.2, §25); it
adds presentation only, never new control semantics.
