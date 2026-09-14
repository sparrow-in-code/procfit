---
id: PM-0312
title: Managed panel as a grouped tree (persist name; orphans group)
state: TODO
phase: 3
depends: ["PM-0311", "PM-0306"]
owner:
rfc: ["§19", "§15"]
---

## Summary

Render the managed panel with the *same* grouping/tree mechanism as the browser:
resolve each managed pid to its live process and group it exactly like the main
view; processes that have exited go under a built-in **orphans** group, shown by
the name they had. Requires persisting each binding's process name so a target is
identifiable after the process is gone.

## Scope

- **Persist identity for display:** store the process name/comm (and pid) with
  each managed binding in the state file, captured at manage time (currently only
  ID + pid + control fields are stored).
- **Grouped tree rendering:** feed the managed pids through the existing query
  grouping/aggregation pipeline (honouring the current group-by/leaf), so the
  managed panel shows the same hierarchy as the browser, restricted to managed
  targets. Control + restore/drop act on the selected node.
- **Orphans group:** a synthetic built-in group collecting bindings whose pid no
  longer resolves (exited / pid reused with different identity), labelled by the
  persisted name, so you can still see and `d`rop/`R`estore them.
- Reuse the shared view component (PM-0311) so browser and managed share the tree
  renderer rather than duplicating it.

## Out of scope

- New control semantics (reuse PM-0303/0306/0307).

## Acceptance criteria

- [ ] Managed bindings persist a display name; it survives process exit.
- [ ] Managed panel renders managed processes grouped like the browser (same
      group-by/leaf), with per-node control/restore/drop.
- [ ] Exited/unresolvable bindings appear under an "orphans" group by their
      persisted name.
- [ ] Live vs orphan is derived from identity revalidation (pid reuse ⇒ orphan).

## Tests required

- unit: state round-trips the persisted name; grouping of managed pids;
  orphan bucketing when a pid doesn't resolve / identity mismatch (fake source).

## Notes

Directly requested: "persist pid + process name, resolve into the same group-tree
when rendering managed processes; if the process is gone, drop them under a
built-in 'orphans' group." Depends on the reusable view (PM-0311) to avoid a
third rendering path.
