---
id: PM-0311
title: Extract a reusable TUI table/view component (browser + managed share it)
state: TODO
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

- [ ] One view component renders both panels; no duplicated row/column/cursor
      code between browser and managed.
- [ ] Legends/keymaps are data per panel, not hardcoded per render function.
- [ ] All existing TUI tests pass unchanged (behaviour preserved); component has
      its own unit tests (nav/scroll/render with a fake item set).

## Notes

Prompted by the observation that the managed panel "could reuse the same view,
only changing actions, legend and items." Also a natural home for the managed
panel to gain the browser's grouping/tree rendering of a target's members.
