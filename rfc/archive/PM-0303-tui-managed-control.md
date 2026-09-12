---
id: PM-0303
title: TUI managed panel, detail view, control preview + confirmation
state: DONE
phase: 3
depends: ["PM-0302", "PM-0205"]
owner:
rfc: ["§19.1", "§19.2", "§19.3"]
---

## Summary

Bring the control side plane into the TUI: manage/nice/stop/freeze/restore on the
selected row, a managed panel, detail overlay, and mandatory action previews.

## Scope

- Managed panel (§19.1) showing retained + INACTIVE targets; `Tab` switches
  panels.
- Control keys (§19.2): `m` manage/unmanage, `n` nice, Space stop/continue, `F`
  freeze/thaw (stub until Phase 5), `R` restore, `i`/Enter inspect.
- Confirmation/preview before group control (§19.3): logical target + binding
  mode, exact match count, protected/skipped count, values to capture/change,
  predicted privilege/restore limits; user can view the dry-run list first.

## Out of scope

- Loop responsiveness (PM-0304); daemon-backed control (Phase 4).

## Acceptance criteria

- [ ] Control actions reuse the Phase 2 controllers/safeguards — no duplicated
      control logic in the TUI.
- [ ] No group control proceeds without the §19.3 preview + confirmation.
- [ ] Managed panel shows INACTIVE targets with last-seen.
- [ ] Predicted restore-privilege warning shown before applying nice.

## Tests required

- unit: preview content assembly; confirmation gating; panel state; action →
  Phase 2 plan wiring (mocked controllers).

## Notes

The TUI is a client of the same control planner as the CLI (§6.2, §25). It adds
presentation, never new control semantics.

## Status: DONE (2026-09-12)

Interactive explorer implemented with tcell: headless unit-testable Model (view state + key handling), tcell driver isolated behind a RefreshFunc callback (UI depends on a query function, not the engine/daemon — framework choice reversible per DEVELOPMENT §2.4), grouping presets, sort toggle, interval step, scrolling, responsive single event+ticker loop (RFC §19.4), and prints the equivalent CLI command on exit (Phase 3 exit criterion). Managed/control panel keys are a follow-up; core browser + query pickers are done.
