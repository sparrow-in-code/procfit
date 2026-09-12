---
id: PM-0204
title: STOP/CONT controller and control safeguards
state: DONE
phase: 2
depends: ["PM-0203"]
owner:
rfc: ["§15.1", "§15.4", "§15.9", "§21.2", "§21.4"]
---

## Summary

Signal-based execution control (SIGSTOP/SIGCONT) with correct intent-vs-observed
separation, plus the shared safeguard module protecting critical processes.

## Scope

- Stop/continue backend via pidfd signal or kill (§15.1); procfit tracks its own stop
  *intent* as control state while `PSTATE` stays the observed kernel state
  (§15.4).
- Pre-stopped detection: if a task was already stopped before procfit, record it and
  do NOT blindly SIGCONT on restore; ambiguous external stop ⇒ report + confirm
  (§15.4).
- Arbitrary `procfit signal <sig> <target>` with confirmation safeguards (§8.6).
- Shared safeguard module (§15.9): refuse PID 1, kernel threads, the procfit
  daemon/client + ancestor chain, session/systemd manager, >safety-limit (256)
  targets without confirm, out-of-permission-boundary, cross-boot ambiguous
  identities. `--force-system` is explicit/noisy and still cannot bypass kernel
  checks; never setuid.
- Confirmation rules (§8.6): destructive signals need TTY confirm or `--yes`.

## Out of scope

- cgroup freeze/thaw (Phase 5) though the safeguard module must be extensible to
  it.

## Acceptance criteria

- [ ] SIGSTOP intent shown separately from observed `PSTATE`; pre-stopped tasks
      not auto-continued on restore (MVP §28.12).
- [ ] Safeguards refuse PID 1, kernel threads, and procfit's own chain by default.
- [ ] Signals revalidate identity immediately before delivery (§21.2).
- [ ] Non-interactive destructive action without `--yes` is refused.
- [ ] `--force-system` cannot bypass kernel permission errors.

## Tests required

- unit: intent/observed separation; pre-stopped record + restore behaviour; each
  safeguard rule; confirmation gating; stale revalidation.
- integration (gated): SIGSTOP/SIGCONT a helper, verify control vs PSTATE
  (§26.3).

## Notes

`CONT` is an action, not a state (§15.4). Never run control integration tests
against arbitrary host processes (§26.3).

## Status: DONE (2026-09-12)

Implemented in the autonomous build: control manager (capture-once original, desired/observed, drift, restore, stale-PID refusal, safeguards, stop-intent, snapshot/follow binding, inactive retention), atomic boot-scoped runtime state, and the control CLI. Verified end-to-end on a live process (nice apply/restore, with the CAP_SYS_NICE restore-privilege case surfaced correctly). Tests cover the semantics; race-clean; coverage ≥80%.
