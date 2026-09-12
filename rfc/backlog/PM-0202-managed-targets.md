---
id: PM-0202
title: Managed targets — bindings, snapshot/follow, inactive retention
state: TODO
phase: 2
depends: ["PM-0201", "PM-0106"]
owner:
rfc: ["§6.5", "§6.6", "§11.4", "§16.6", "§8.5"]
---

## Summary

The logical-target layer: persistent/manual targets that bind to zero-or-more
live instances, remain visible when inactive, and re-bind (follow) or not
(snapshot).

## Scope

- `ManagedTarget` model (§11.4): id, name, origin, binding mode, compiled
  selector, snapshot ids, control intent, last-seen, counters.
- Binding resolution against current entity generation; follow re-resolves,
  snapshot binds only instances resolved at creation (§8.6 defaults).
- Managed-membership-without-control (§6.6): membership does not imply a control
  action.
- Inactive retention (§16.6): vanished instances leave the live tree immediately
  but the logical target persists as `INACTIVE` with last-seen/counters; follow
  re-binds future matches, snapshot never re-binds.
- Target syntax parsing (§8.5): `pid:`, `tid:`, `managed:`, `group:`,
  `selector:`; bare numeric = PID; no silent string guessing.
- Persistent `managed[].selector` uses stable-identity fields only; rate metrics
  rejected by default (§6.5).

## Out of scope

- Actual control application (PM-0203/PM-0204); daemon reconciliation (Phase 4).

## Acceptance criteria

- [ ] `manage` without control creates membership only (§6.6, §8.6).
- [ ] A disappeared PID leaves the live view immediately while its follow target
      stays `INACTIVE` (MVP §28.13).
- [ ] Snapshot target never re-binds; follow target re-binds new matches.
- [ ] Persistent selector referencing a rate metric is rejected (§6.5).
- [ ] Target-syntax parser rejects ambiguous bare strings (§8.5).

## Tests required

- unit: binding resolution; snapshot vs follow; inactive retention +
  counters/last-seen; target syntax parsing; rate-in-selector rejection.

## Notes

This is the pivot between the observation plane and the control side plane
(§6.2). Managed membership recorded here is what control commands act on.
