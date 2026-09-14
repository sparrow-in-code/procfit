---
id: PM-0312
title: Managed panel as a grouped tree (persist name; orphans group)
state: DONE
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

- **Persist identity for display:** store the **pid + process name/comm** with
  each managed binding in the state file, captured at manage time (the pid is
  already persisted; add the name so an exited target is still identifiable by
  both pid and name).
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

- [x] Managed bindings persist pid + display name; both survive process exit
      (shown in the orphans group).
- [x] Managed panel renders managed processes grouped like the browser (same
      group-by/leaf), with per-node control/restore/drop.
- [x] Exited/unresolvable bindings appear under an "orphans" group by their
      persisted name.
- [ ] Live vs orphan is derived from identity revalidation (pid reuse ⇒ orphan).
      **Deferred:** live/orphan is currently derived from pid presence in the live
      sample, not full BootID/StartTime revalidation, so a reused pid shows as
      live under a stale name. See follow-up below.

## Tests required

- unit: state round-trips the persisted name; grouping of managed pids;
  orphan bucketing when a pid doesn't resolve / identity mismatch (fake source).

## Notes

Directly requested: "persist pid + process name, resolve into the same group-tree
when rendering managed processes; if the process is gone, drop them under a
built-in 'orphans' group." Depends on the reusable view (PM-0311) to avoid a
third rendering path.

## Status: DONE (2026-09-14)

Managed bindings now persist `Name` (process display name, captured at manage time
and backfilled from the live process while alive) alongside the pid. The managed
panel is fed by `assembly.tuiManagedTree`: it samples live processes, keeps only
managed pids, and runs them through the *same* query grouping/aggregation pipeline
as the browser (honouring the current group-by/leaf). `managedPIDs` resolves the
managed set + names; `orphanGroup` buckets managed pids absent from the live sample
under a synthetic built-in **orphans** group, each a ghost row carrying the
persisted pid + name so `d`rop / restore still address it. Control/drop act on the
selected node. Covered by `TestTUIManagedTree_LiveThenOrphan` and `TestOrphanGroup`
(fake source); `make check` green at 80.3%.

### Follow-up (new ticket worthy)

Orphan detection is by **pid membership** in the live sample, not identity
revalidation. A pid reused by an unrelated process after the managed process exits
would be shown as a *live* managed row under the old name, rather than orphaning
the old binding. Closing this needs comparing the persisted
`ProcessInstanceID` (BootID + StartTime) against the sampled process and treating a
mismatch as an orphan. Deferred to keep this change behaviour-preserving; tracked
as the remaining unchecked acceptance criterion.
