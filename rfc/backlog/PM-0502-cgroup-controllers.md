---
id: PM-0502
title: cgroup v2 freeze/thaw, CPU, and I/O controllers
state: TODO
phase: 5
depends: ["PM-0204"]
owner:
rfc: ["§15.1", "§15.5", "§15.9"]
---

## Summary

cgroup v2-backed controls plus ioprio and CPU-affinity, extending the control
side plane with the same original/desired/observed discipline.

## Scope

- Freeze/thaw via cgroup v2 `cgroup.freeze` (§15.1) — **only** on an existing
  delegated/writable cgroup (§15.5); no automatic migration.
- CPU weight/max and I/O controls via cgroup v2; ioprio via ioprio_get/set;
  CPU affinity via sched_get/setaffinity (§15.1).
- Each controller declares whether it edits an existing cgroup in place or would
  migrate (migration excluded here, §15.5).
- Reuse control-state tuple, drift, restore, safeguards from Phase 2.
- `--freeze/--thaw` flags (stubbed in PM-0205) now functional.

## Out of scope

- Automatic process migration into pm-owned cgroups (open question §30.10).

## Acceptance criteria

- [ ] Freeze operates only on delegated/writable cgroups; refuses otherwise.
- [ ] Each new control records original/desired/observed and restores safely.
- [ ] cgroup writes lacking delegation ⇒ permission_denied, not silent failure.
- [ ] Safeguards (§15.9) apply to cgroup targets too.

## Tests required

- unit: freeze/thaw state machine; delegation check; ioprio/affinity capture +
  restore; drift.
- integration (gated): cgroup v2 freeze only when delegated (§26.3).

## Notes

Migration changing cgroup membership conflicts with systemd/containers (§15.5)
— explicitly out of scope; needs its own RFC before implementation (§30.10).
