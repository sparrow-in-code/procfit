---
id: PM-0311
title: Extract a reusable TUI table/view component (browser + managed share it)
state: DONE
phase: 3
depends: ["PM-0303", "PM-0306"]
owner:
rfc: ["§19"]
---

## Summary

The managed panel is currently a parallel rendering + keymap path
(`managedFrame`/`managedLine`/`managedHeader`/`managedKey`) that duplicates the
browser's row/column/cursor/scroll machinery. Extract a single reusable view
component parameterized by its data, columns, keymap/legend, and actions, so the
browser and managed panel (and any future panel) are instances of it.

## Scope

- A `tableView` (or similar) owning: rows, columns, cursor, scroll, header,
  status/legend, and generic navigation — no knowledge of processes vs targets.
- Parameterize per panel:
  - item provider (rows + how to render each column),
  - keymap → actions + the legend string,
  - optional tree/fold support (browser) vs flat (managed).
- Reimplement the browser and managed panel as configurations of the component;
  delete the duplicated `managed*` rendering.
- Keep the headless, unit-testable Model design (DEVELOPMENT §2.4).

## Out of scope

- New features; this is a structural refactor (behaviour-preserving).

## Acceptance criteria

- [x] One view component renders both panels; no duplicated row/column/cursor
      code between browser and managed.
- [x] Legends/keymaps are data per panel, not hardcoded per render function.
- [x] All existing TUI tests pass unchanged (behaviour preserved); component has
      its own unit tests (nav/scroll/render with a fake item set).

## Notes

Prompted by the observation that the managed panel "could reuse the same view,
only changing actions, legend and items." Also a natural home for the managed
panel to gain the browser's grouping/tree rendering of a target's members.

## Status: DONE (2026-09-14)

The managed panel no longer has its own render/keymap path: the duplicated
`ManagedRow`/`managedFrame`/`managedHeader`/`managedKey` code is gone. Both panels
are now the same browser view (`Frame`) fed a different `RefreshFunc` provider and
a per-panel legend; the driver picks the provider per active panel. `tui.Deps`
carries `Refresh` (browser) and `ManagedTree` (managed) as interchangeable
`RefreshFunc`s, plus `Drop` for the managed-only `d` key. Verified via the headless
Model tests (rewritten `TestModel_ManagedPanel`,
`TestModel_ManagedControlActsOnTarget`) and `make check` green. Landed together
with PM-0312, which supplies the managed provider.
