---
id: PM-0314
title: Fold/unfold all groups at once (key + --collapse-groups flag/config)
state: DONE
phase: 3
depends: ["PM-0306"]
owner:
rfc: ["§19", "§9"]
---

## Summary

Give the interactive explorer a one-shot way to open or close every group in the
current tree, and a launch-time default for the same. Deep grouped trees are
tedious to fold one node at a time; `c`/`C` fold/unfold all live, and
`--collapse-groups` (config `collapse_groups`, env `PROCFIT_COLLAPSE_GROUPS`)
starts the TUI fully folded so the user opens only what they care about.

## Scope

- **In-TUI keys:** `c` folds every group shut, `C` unfolds every group. Both are
  view-only (no re-query; work while paused). A per-group `Enter`/`←`/`→` override
  still wins afterwards, and groups that appear later (regroup/refresh) follow the
  last fold-all default.
- **Launch flag / config / env:** `--collapse-groups` bool, wired through the
  shared query-flags + config precedence machinery (like `target-width`):
  `collapse_groups` in config, `PROCFIT_COLLAPSE_GROUPS` env, shown by
  `--show-config`, round-tripped by `config dump`/`convert`.
- Model change: per-group `collapsed` map becomes explicit *overrides* over a
  `foldDefault`; `isCollapsed(path)` resolves override-then-default; `foldAll`
  flips the default and clears overrides.

## Out of scope

- `ps`/`stat` honouring the flag (they accept but ignore it today — see QUESTIONS
  item H; a "group summary" ps view could honour it later).

## Acceptance criteria

- [x] `c`/`C` fold/unfold all groups at once; view-only (no re-query), overrides
      still work, new groups follow the default.
- [x] `--collapse-groups` starts the TUI with all groups folded.
- [x] Config (`collapse_groups`) + env (`PROCFIT_COLLAPSE_GROUPS`) + `--show-config`
      resolve with the standard precedence; `config dump` round-trips it.

## Tests required

- unit: `TestModel_FoldAll` (c/C, view-only, default persists to new groups),
  `TestModel_CollapseGroupsLaunchFlag` (flag + per-group override);
  `TestApplyConfigDefaults_CollapseGroups` (config→flag, env, CLI precedence).

## Notes

`foldDefault` + override map keeps fold-all O(1) and makes "new groups follow the
default" fall out naturally, rather than walking the tree to stamp every path.
`make check` green (coverage 80.3%). Direct user request.
