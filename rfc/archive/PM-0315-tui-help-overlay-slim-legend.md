---
id: PM-0315
title: In-TUI help overlay (?) + slim status bar
state: DONE
phase: 3
depends: ["PM-0303", "PM-0311"]
owner:
rfc: ["§19"]
---

## Summary

The status-bar legend grew a token per key until it no longer fit and scanned
poorly. Replace the inline keymap with a vim/tmux-style help overlay on `?`, and
trim the steady-state status line to the essentials, so it stops growing as keys
are added.

## Scope

- **Slim status bar:** name (+`MANAGED` tag on the managed panel), `gen`/`rows`,
  interval (or `PAUSED`), one nav hint, `[?]help [q]uit`, then the status outlet.
- **Help overlay (`?`):** full-screen keymap grouped into NAVIGATION / VIEW /
  CONTROL / PANELS / GENERAL. Control and panel sections appear only when a
  controller / managed source is wired. Closes on `?`/`Esc`/`q`.
- **Short-terminal friendly:** the overlay scrolls (`↑`/`↓`/`PgUp`/`PgDn`,
  clamped) and advertises "(↑/↓ scroll)" when the keymap doesn't fit.

## Out of scope

- Mouse support; per-key remapping/config.

## Acceptance criteria

- [x] Status bar no longer inlines the full keymap; shows only name, gen/rows,
      interval, nav hint, `[?]help [q]uit`, status.
- [x] `?` opens/closes a help overlay listing all controls; Esc/q also close.
- [x] Overlay adapts to availability (no CONTROL/PANELS sections when absent).
- [x] Overlay scrolls and clamps on a terminal too short to show it all.

## Tests required

- unit: `TestModel_SlimStatusBar`, `TestModel_HelpOverlay`,
  `TestModel_HelpAdaptsToAvailability`, `TestModel_HelpScrollsOnShortTerminal`.

## Notes

The table render loop already paints only the visible slice (`bodyHeight` rows
from `scroll`), and WINCH is handled via tcell `EventResize` → `SetSize` with
`bodyHeight` floored at 1 — so "don't render all rows on a small screen" is
already true; the overlay reuses the same windowing. `make check` green (80.5%).
Direct user request.
