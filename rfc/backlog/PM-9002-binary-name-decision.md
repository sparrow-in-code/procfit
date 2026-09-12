---
id: PM-9002
title: Decision — final project/binary name and canonical metric names
state: TODO
phase: 0
depends: []
owner:
rfc: ["§13", "§30.1", "§30.3"]
---

## Summary

Track and resolve the naming open questions so they do not block, but are decided
before v1: the binary name (`procfit` is provisional and may collide) and canonical
disk-metric names.

## Scope

- Decide final binary/project name (§30.1). Until then, keep the name behind a
  single constant (see PM-0001) so renaming is trivial.
- Decide canonical disk metric ids: long (`disk-read-bytes`) vs interactive
  (`disk-rbps`); RFC §30.3 recommends the shorter forms. Record aliases (§13.1).
- Capture each decision as an amendment note appended to this ticket and folded
  into the master RFC.

## Out of scope

- Any code change beyond the name constant (owned by PM-0001/PM-0003).

## Acceptance criteria

- [ ] A recorded decision for the binary name (or explicit "keep `procfit` for now").
- [ ] A recorded decision fixing canonical disk-metric ids + aliases before v1.
- [ ] Master RFC updated to reflect both decisions.

## Tests required

- none (decision ticket); ensure registry tests (PM-0003) assert the chosen
  canonical ids/aliases once decided.

## Notes

This is a living decision ticket; it may sit in `backlog/` until v1 approaches.
Other open questions (§30.2, §30.4–30.10) are tracked inline in their respective
implementation tickets.
