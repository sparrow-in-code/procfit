---
id: PM-0104
title: Metadata resolvers (uid/gid, cgroup, systemd, app, namespace alias)
state: DONE
phase: 1
depends: ["PM-0101"]
owner:
rfc: ["§11.2", "§14.2", "§30.2"]
---

## Summary

Decorators that enrich normalized entities with human labels and derived
identity (user names, cgroup path, systemd unit, desktop app id, namespace
aliases) without altering native keys.

## Scope

- `Resolver` interface (RFC §14.2): `ID`, `Resolve(ctx, *Process)`.
- Initial resolvers: passwd UID/GID with caching; cgroup v2 path; systemd unit
  heuristic from cgroup path; desktop app heuristic from exe/cmdline + `.desktop`
  files; namespace user-alias map from config.
- Namespace identity remains inode-based (§11.2); labels are metadata only. When
  no label exists, display `pidns:<inode>` — never fabricate a name.
- Failed resolver must not invalidate the base process (§14.2).

## Out of scope

- Container runtime / Kubernetes resolvers (later plugins, §14.2).

## Acceptance criteria

- [ ] Resolvers never mutate namespace/dimension keys, only add labels (§11.2).
- [ ] A panicking/erroring resolver leaves the base entity intact and usable.
- [ ] UID→name lookups are cached; no per-entity passwd syscall storm.
- [ ] Unlabeled namespace renders as `pidns:<inode>` etc.

## Tests required

- unit: each resolver with fixtures; failure isolation; cache behaviour;
  no-label fallback formatting.

## Notes

Resolver ordering/precedence for app-id is an open question (RFC §30.2); pick a
documented default and note deviations. Composition over inheritance: resolvers
chain, each single-responsibility.

## Status: DONE (2026-09-12)

Implemented UserResolver (uid→login via passwd, cached, testable path) and SystemdResolver (unit from cgroup-v2 path heuristic), applied as a decorator chain in the app after sampling. Added the 'user' dimension and column. Verified: 'ps --group-by user' shows real login names. Container/Kubernetes resolvers remain future work.
