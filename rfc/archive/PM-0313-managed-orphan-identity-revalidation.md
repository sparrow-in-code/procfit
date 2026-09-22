---
id: PM-0313
title: Orphan managed bindings on pid reuse (identity revalidation)
state: DONE
phase: 3
depends: ["PM-0312"]
owner:
rfc: ["§19", "§15"]
---

## Status: DONE (2026-09-22)

`assembly.managedPIDs` now returns each pid's persisted `ProcessInstanceID`
(not a bare `map[int]bool`); the live/orphan split in `tuiManagedTree` counts a
sampled process as a live managed row only when `binding.ID.SameProcess(p.ID)`,
so a reused pid (same pid, different BootID/StartTime) fails the check → the old
binding falls under **orphans** and the unrelated new process is not shown as
managed. Name backfill is likewise gated on an identity match, so an orphan keeps
its own captured name. Control actions from the managed panel already revalidate
identity in the manager (`Manager.revalidate` on every nice/stop/restore/signal),
so a reused pid is refused (`StatusStale`) rather than controlled under the stale
target — no change needed there, covered by existing `control` tests.
Tests: `app.TestTUIManagedTree_PidReuseOrphans` (reused pid ⇒ orphan, new process
not managed); existing `TestTUIManagedTree_LiveThenOrphan`/`TestOrphanGroup`/
`TestManagedPIDs_Backfill` updated for the identity-aware signature.
Commit: (local).

## Summary

The managed panel (PM-0312) decides live-vs-orphan by **pid membership** in the
live sample: a managed pid present in the current sample is shown as a live row,
otherwise it goes under the built-in **orphans** group. This misclassifies pid
reuse — after the managed process exits and the kernel reassigns its pid to an
unrelated process, the managed binding is shown as *live* under the old name
instead of being orphaned.

Each binding already persists the full `ProcessInstanceID` (BootID + StartTime);
use it to revalidate identity when resolving managed pids, so a mismatch orphans
the old binding (and never renices/stops the wrong process).

## Scope

- In `assembly.managedPIDs` (and the live/orphan split in `tuiManagedTree`),
  match a managed binding to a sampled process by **`ProcessInstanceID`**, not pid
  alone: a pid present but with a different BootID/StartTime is treated as **not
  live** (the old binding orphans; the new process is unmanaged).
- Ensure control actions from the managed panel also revalidate identity before
  acting, so a reused pid is never controlled under a stale target.
- Keep the orphan ghost row labelled by the persisted name/pid as today.

## Out of scope

- Automatically dropping orphaned bindings (they remain until `d`ropped).

## Acceptance criteria

- [x] A managed pid that is reused by a process with a different
      `ProcessInstanceID` appears under "orphans", not as a live managed row.
- [x] Control/restore from the managed panel refuses (or re-resolves) when the
      live pid's identity no longer matches the persisted binding.
- [x] Existing live/orphan behaviour for genuine exit is unchanged.

## Tests required

- unit: fake source where a managed pid is re-added with a different StartTime;
  assert the old binding orphans and the new process is not managed.

## Notes

Deferred from PM-0312 to keep that change behaviour-preserving; it is the one
unchecked acceptance criterion there ("live vs orphan derived from identity
revalidation, pid reuse ⇒ orphan").
