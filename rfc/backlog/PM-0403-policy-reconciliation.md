---
id: PM-0403
title: Config managed selectors + continuous reconciliation
state: TODO
phase: 4
depends: ["PM-0402", "PM-0203"]
owner:
rfc: ["§6.5", "§6.6", "§15.7", "§17.1"]
---

## Summary

Persistent policy: config-defined managed selectors that the daemon continuously
matches and reconciles, capturing per-instance originals for new matches.

## Scope

- Load `[[managed]]` rules from config (§6.6) as follow-mode policy targets;
  membership vs `[managed.control]` control distinction preserved.
- Continuous reconciliation on each fresh entity generation (§15.7): new matching
  instances capture their own original before policy application.
- Per-rule `on_drift` behaviour (§15.7), `reapply` rate-limited + event-emitting.
- Config policies always follow (§8.6); persistent selectors restricted to
  stable-identity fields (§6.5).

## Out of scope

- Reload mechanics + systemd unit (PM-0404).

## Acceptance criteria

- [ ] A policy matches a newly-started process after the daemon is running
      (§27.Phase4 exit).
- [ ] New matches capture original before policy applies (§15.7).
- [ ] `reapply` is rate-limited and emits an event (no silent controller fights,
      §15.7).
- [ ] Managed-only rules (no control block) enforce nothing (§6.6).

## Tests required

- unit: reconciliation with a stream of entity generations (fake clock); drift
  modes; original-capture on new match; rate-limit.
- integration (gated): start daemon, spawn matching helper, verify policy
  application + drift handling.

## Notes

Reconciliation is the reason the daemon exists (§17.1). Keep it in the control
plane; it must not alter the observation pipeline.
