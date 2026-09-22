---
id: PM-0519
title: TUI interactive column add/remove (no restart)
state: DONE
phase: 3
depends: ["PM-0518"]
owner:
rfc: ["§19"]
---

## Summary

Adding a column meant restarting with `--columns`. Let the explorer add/remove
columns live: `+` opens a prompt (type an id, prefix-matched against the field
catalog, Enter appends it to the right), `-` drops the rightmost. A re-query then
renders the new set.

## Scope

- `+` add-column prompt (`colEditing`/`colBuf`), prefix-match against the addable
  ids (every metric + structural column, `SetColumnChoices` from `Deps.Columns`);
  Enter appends to the current columns, `-` removes the last (never the last one).
- Materialize the displayed columns to an explicit `flags.Columns` before editing
  (columns otherwise come implicitly from the profile), then mark dirty.
- Status-bar prompt shows the live match; help overlay + README document it.

## Out of scope

- Reordering / insert-at-position / a full multi-select picker (append-to-right
  only for now); per-panel column sets.

## Acceptance criteria

- [x] `+` then a prefix + Enter appends the matching column and re-queries.
- [x] `-` drops the rightmost column; never removes the last remaining one.
- [x] Explicit column edits stick across grouping/sort changes; no restart.

## Tests required

- unit: TestModel_AddRemoveColumn (prompt open, prefix match, append/remove, dirty).

## Status: DONE (2026-09-22)

`internal/tui/columns.go` holds the add/remove logic; `+`/`-` bound in update.go,
routed like the filter editor; prompt rendered in the status bar; choices sourced
from `assembly.addableColumns()` (metrics + `render.StructuralColumns()`). Reuses
the existing "flags change → re-query" path, so no new refresh plumbing.
`make check` green (80.5%).
