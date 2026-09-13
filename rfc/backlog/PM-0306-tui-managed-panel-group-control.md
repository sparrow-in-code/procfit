---
id: PM-0306
title: TUI managed panel, detail overlay, and group control preview
state: TODO
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
- [ ] Group/selector control shows the §19.3 preview (match/protected counts,
      capture/change values, privilege forecast) before applying.
- [ ] Detail overlay shows identity + control history for the selected row.
- [ ] Control still reuses the Phase-2 controllers/safeguards (no duplicated
      control logic in the TUI).

## Tests required

- unit: managed-panel state + Tab switching; group-preview content assembly;
  privilege forecast; detail overlay rendering (headless Model, mocked
  controller).

## Notes

The TUI remains a client of the same control planner as the CLI (§6.2, §25); it
adds presentation only, never new control semantics.
