---
id: PM-0505
title: Optional persistent audit/history
state: TODO
phase: 5
depends: ["PM-0201"]
owner:
rfc: ["§16.1", "§16.6", "§23", "§30.7"]
---

## Summary

Opt-in persistent history: audit events and inactive-target lifetime counters in
`$XDG_STATE_HOME/pm/`, kept separate from runtime state.

## Scope

- Audit event log + inactive-target counters (§16.1) as persistent, opt-in
  (`[state].history`).
- History must NOT rebind by numeric PID or namespace inode alone across boots
  (§16.5/§16.6).
- Optional `pm doctor` groundwork: redact cmdlines/paths by default (§23).

## Out of scope

- Long-term time-series metrics (explicit non-goal §4).

## Acceptance criteria

- [ ] History is opt-in and off by default; disabled ⇒ no history I/O.
- [ ] History never causes PID/inode-based rebinding across boots.
- [ ] Any diagnostic export redacts cmdlines/paths by default (§23).

## Tests required

- unit: event append + retention; opt-in gating; cross-boot no-rebind;
  redaction.

## Notes

Storage format + retention is an open question (§30.7) — record the decision in a
PM-90NN note. Keep history strictly separate from runtime state (§16.1).
