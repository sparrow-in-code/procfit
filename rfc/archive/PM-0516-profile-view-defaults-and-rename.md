---
id: PM-0516
title: Profiles carry a default view; rename --metrics to --profile
state: DONE
phase: 3
depends: ["PM-0512", "PM-0513"]
owner:
rfc: ["§8", "§13.5"]
---

## Summary

Let a built-in profile predefine not just its metrics but its default **view** —
column set + order, sort, grouping, and filter — applied unless the user overrides
them. Rename the `--metrics` flag to the clearer `--profile` (keeping `--metrics`
as a deprecated alias).

## Scope

- Change the profile spec from `[]MetricID` to a `Profile{Metrics, Columns, Sort,
  GroupBy, Having}`; `ProfileDefaults(name)` exposes the view.
- `queryspec.Build` fills empty Sort/GroupBy/Having from the profile
  (`applyProfileDefaults`) and uses the profile's explicit ordered Columns (which
  may include non-metric fields like pstate/wchan) verbatim; explicit
  user/config flags always win.
- memory profile: sort `pss:desc` + curated columns; sysload profile: sort
  `runq-delay:desc` + `pstate`/`wchan` columns.
- Rename flag to `--profile` (viewSettings key, env `PROCFIT_PROFILE`); keep
  `--metrics` bound to the same value and folded into the args layer for correct
  precedence.

## Acceptance criteria

- [x] A profile can set default columns/sort/group-by/having; explicit flags
      override each independently.
- [x] `--profile` is the primary flag; `--metrics` still works (deprecated) and
      the CLI value wins over config; `--show-config` shows `profile`.

## Tests required

- unit: profile view-defaults applied + overridden; --metrics alias precedence.

## Status: DONE (2026-09-22)

`metrics.Profile` struct + `ProfileDefaults`; `applyProfileDefaults` and the
explicit-columns path in `chooseColumns`. Flag renamed with the alias folded in
`applyConfigDefaults`. Verified live: `--profile sysload` sorts by runq-delay with
pstate/wchan columns, `--profile memory` sorts by pss, `--metrics` alias +
`--show-config` (profile/PROCFIT_PROFILE) work, explicit `--sort` overrides.
`make check` green (80.5%).
