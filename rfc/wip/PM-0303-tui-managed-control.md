---
id: PM-0303
title: TUI managed panel, detail view, control preview + confirmation
state: WIP
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

- [x] Control actions reuse the Phase 2 controllers/safeguards — no duplicated
      control logic in the TUI. (TUI calls a `ControlFunc` that drives the same
      `control.Manager` path as `set`/`restore`.)
- [x] No control proceeds without a preview + confirmation. (Every action shows a
      dry-run/synthesized preview and requires `y`; nice has a real manager
      dry-run surfacing safeguards. Group control in the TUI is out of this slice.)
- [ ] Managed panel shows INACTIVE targets with last-seen. → deferred to PM-0306.
- [ ] Predicted restore-privilege warning shown before applying nice. → partial;
      the nice dry-run surfaces protected/denied per pid; a dedicated privilege
      forecast is deferred to PM-0306.

## Tests required

- unit: preview content assembly; confirmation gating; panel state; action →
  Phase 2 plan wiring (mocked controllers).

## Notes

The TUI is a client of the same control planner as the CLI (§6.2, §25). It adds
presentation, never new control semantics.

## Status

**Reopened 2026-09-13.** This ticket was previously archived as DONE while its
acceptance criteria were unmet and no control code existed in `internal/tui/` —
only the observation explorer (PM-0301/0302/0304) had shipped.

**Now landed (this slice):** single-process control from the TUI — `n` renice
(numeric prompt), `x`/`c` stop/continue, `z`/`Z` freeze/thaw, `R` restore — each
behind a mandatory preview + `y` confirmation, wired via a `tui.ControlFunc`
that reuses the CLI's `control.Manager` (safeguards, dry-run, state save). Unit
tests cover the confirm gating and preview→apply wiring with a fake controller.

**Deferred to PM-0306:** the managed panel (retained/INACTIVE targets, `Tab`),
detail overlay (`i`/Enter inspect), group/selector control from the TUI with the
full §19.3 group preview, and the dedicated restore-privilege forecast.
