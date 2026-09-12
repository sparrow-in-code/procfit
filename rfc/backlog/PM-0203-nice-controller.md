---
id: PM-0203
title: Nice controller — original/desired/observed, drift, restore
state: TODO
phase: 2
depends: ["PM-0202"]
owner:
rfc: ["§6.2", "§15.1", "§15.2", "§15.3", "§15.6", "§15.7", "§15.8", "§21.2"]
---

## Summary

The first real controller and the full control-state machine: capture original
once, track desired/observed, detect drift, restore safely. Establishes the
side-plane pattern all controllers follow.

## Scope

- `Controller` interface (§25): Plan/Apply/Restore/Observe.
- Nice backend via getpriority/setpriority (§15.1).
- `ControlledField[T]` tuple (§15.2): capture Original exactly once before first
  successful mutation; desired update must not overwrite original.
- Identity revalidation before every mutation (§21.2): pidfd where available,
  else re-read pid+starttime; stale ⇒ mark stale, never act on reused PID.
- Multi-instance apply (§15.6): resolve, sort, exclude protected, revalidate,
  capture, apply, immediately observe, persist, return per-instance result
  counts (applied/unchanged/skipped/stale/denied/failed/vanished — §21.4).
- Drift modes (§15.7): report(default)/reapply(rate-limited)/adopt/ignore.
- Restore vs unmanage (§15.8): restore is compare-and-set to original; refuses to
  overwrite drift (exit 7) unless `--force`.
- Predict restore-needs-privilege and warn before applying (§15.3, CAP_SYS_NICE).

## Out of scope

- STOP/CONT (PM-0204); cgroup/CPU/IO controllers (Phase 5); safeguards ticket
  (PM-0204 owns the shared safeguard module — coordinate).

## Acceptance criteria

- [ ] Nice records original/desired/observed and can restore original when
      permitted (MVP §28.10).
- [ ] External nice change ⇒ drift, not silent history rewrite (§28.11).
- [ ] Stale PID (starttime mismatch) ⇒ action refused (§28.8, §21.2).
- [ ] Restore over drifted state returns exit 7 unless forced.
- [ ] Multi-instance apply returns structured per-binding results (§21.4).

## Tests required

- unit: original-capture-once; desired/observed transitions; each drift mode;
  restore compare-and-set; stale refusal; privilege-prediction warning.
- integration (gated): set+restore nice on a spawned helper; simulated external
  drift (§26.3).

## Notes

Control is a **side plane** (§6.2): observation must keep working when control is
unavailable. Query rows are only *annotated* from control state.
