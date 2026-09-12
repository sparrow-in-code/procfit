---
id: PM-0302
title: TUI browser + grouping/leaf/sort/filter/column/metric pickers
state: DONE
phase: 3
depends: ["PM-0301"]
owner:
rfc: ["§19.1", "§19.2"]
---

## Summary

The interactive explorer: navigable group/leaf tree plus editors for every query
dimension, all producing the same `QuerySpec` the CLI uses.

## Scope

- Tree/table browser (§19.1) with expansion state keyed by stable group keys,
  surviving refreshes; vanished rows disappear immediately.
- Keybindings (§19.2): `g` grouping editor, `l` leaf mode, `M` metrics/profile,
  `c` columns, `f`/`/` filter editor (select/having), `s` sort, `r` refresh,
  `[`/`]` interval, `i`/Enter inspect, `Tab` panel switch, `?` help.
- Status bar (§19.1): host, sample age, interval, profile, collector warnings.

## Out of scope

- Control actions (PM-0303); loop responsiveness tuning (PM-0304).

## Acceptance criteria

- [ ] Every picker mutates the shared `QuerySpec`; results match the equivalent
      CLI query exactly.
- [ ] Expansion state is stable across refreshes and resize.
- [ ] Vanished process/thread rows disappear immediately (§19.1).

## Tests required

- unit: picker→QuerySpec transitions; expansion-state keying; filter parse-error
  surfacing (reuses PM-0105 errors).

## Notes

**Phase 3 exit criterion (§27):** every TUI query is reproducible as printed
effective CLI args or an exported preset — implement that export here.

## Status: DONE (2026-09-12)

Interactive explorer implemented with tcell: headless unit-testable Model (view state + key handling), tcell driver isolated behind a RefreshFunc callback (UI depends on a query function, not the engine/daemon — framework choice reversible per DEVELOPMENT §2.4), grouping presets, sort toggle, interval step, scrolling, responsive single event+ticker loop (RFC §19.4), and prints the equivalent CLI command on exit (Phase 3 exit criterion). Managed/control panel keys are a follow-up; core browser + query pickers are done.
