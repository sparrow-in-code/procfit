---
id: PM-0517
title: TUI groups by any displayed dimension column (+ state dimension)
state: DONE
phase: 3
depends: ["PM-0515"]
owner:
rfc: ["§19", "§12"]
---

## Summary

The TUI `g` cycle was a fixed preset list, so newly-useful dimensions (notably
`wchan`, and process `state`) were unreachable interactively. Make the cycle
dynamic: the built-in presets plus any *displayed* column that is itself a group
dimension. Metrics (cpu/rss/…) stay out — a continuous value isn't a bucket.

## Scope

- `render.Column.Groupable` flag; set on the categorical/identity columns that map
  1:1 to a group dimension: comm, name, user, uid, ppid, pstate, wchan.
- New `pstate` group dimension (state code R/S/D/…), so `--group-by pstate` and the
  TUI both work.
- TUI `cycleGroup` rebuilds the rotation each press: presets ∪ displayed groupable
  columns; current position found by group-by id (removed the static groupIx).

## Acceptance criteria

- [x] Displaying a `wchan`/`pstate` column makes `g` offer grouping by it; a
      metric column never becomes groupable.
- [x] `--group-by pstate` works on the CLI too.

## Tests required

- unit: group cycle contains displayed dimension columns, excludes metric columns,
  and `g` reaches wchan grouping.

## Status: DONE (2026-09-22)

`Groupable` on the dimension-backed structural columns; `pstate` dimension added;
`cycleGroup`/`groupCycle` derive the rotation from `groupPresets` + displayed
groupable columns. So in the `sysload` view (which shows pstate + wchan), `g`
cycles to "by wchan"/"by pstate". Help/README updated. Verified live
(`--group-by pstate` buckets by state). `make check` green (80.5%).
