---
id: PM-0520
title: HOSTNAME column/dimension from process environ (poor-man's container name)
state: DONE
phase: 5
depends: ["PM-0515"]
owner:
rfc: ["§11", "§12", "§13"]
---

## Summary

Add a `hostname` field: the process's `HOSTNAME` environment variable (from
`/proc/PID/environ`), falling back to the host's own hostname when unset or the
environ is unreadable. Containers typically set `HOSTNAME` to the container
id/name, so grouping by it gives a poor-man's per-container view without any
container-runtime integration. Displayable, sortable, groupable, and filterable
like `wchan`.

## Scope

- New `Hostname` field on `model.Process` + `ports.ProcStat`.
- procfs adapter: lazy `SetReadHostname` gate (off by default; environ is
  permission-gated and adds a read per process); parse NUL-separated environ for
  `HOSTNAME`; host fallback read once from `/proc/sys/kernel/hostname` (honours
  `WithRoot`) → `os.Hostname()`.
- Register as a structural column (`hostname`), a group-by dimension, and in the
  select/having allow-lists; wire the per-query lazy read via `configureSource`
  (generalized `usesWchan`→`usesField`).

## Out of scope

- Full container/pod identity resolution (`container`/`pod` dimensions already
  exist for cgroup/runtime-derived ids); this is the env-var heuristic only.

## Acceptance criteria

- [x] `hostname` appears in `procfit metrics` with `cgsf` USE flags.
- [x] A process whose environ sets `HOSTNAME` reports that value; one without
      falls back to the host hostname; never an empty/zero when requested.
- [x] Sortable, group-by-able, and usable in `select`/`having`.
- [x] Off by default — no environ read unless the query references `hostname`.

## Tests required

- unit: `procfs.TestSource_Hostname` (lazy off-by-default; env value wins;
  host-hostname fallback via fixture `sys/kernel/hostname`).
- unit: `query.TestDimensions_HostnameGroups` + IDs guard includes `hostname`.

## Notes

Mirrors the `wchan` lazy-field seam (PM-0515). Fallback is applied in the source
so grouping is stable (empty never groups, but the source fills the host name
before that stage). Reads environ per process only when demanded.
