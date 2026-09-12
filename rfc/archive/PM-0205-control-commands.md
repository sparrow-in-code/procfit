---
id: PM-0205
title: Control CLI — manage/set/restore/unmanage/signal, managed, inspect
state: DONE
phase: 2
depends: ["PM-0203", "PM-0204"]
owner:
rfc: ["§7.4", "§8", "§8.6", "§8.7", "§21.4"]
---

## Summary

Wire the control side plane to CLI commands, completing the MVP (Phases 0–2).

## Scope

- Commands: `managed [--all]`, `inspect <target>`, `manage <target...>`,
  `set <target...>`, `restore <target...>`, `unmanage <target...>`,
  `signal <sig> <target>` (§8).
- Control flags (§8.6): `--nice`, `--stop/--continue`, `--freeze/--thaw`
  (stubbed until Phase 5), `--snapshot/--follow`, `--dry-run`, `--yes`,
  `--force-system`, `--on-drift`, `--field` for restore.
- Binding-mode defaults (§8.6): pid/tid ⇒ snapshot; managed/group/selector ⇒
  follow.
- `--dry-run` shows exact resolved instances + required capabilities (§15.6).
- Managed view (§7.4) incl. INACTIVE targets with last-seen.
- Exit codes per §8.7, including 6 (partial) and 7 (drift/stale); machine output
  carries per-target errors (§8.7, §21.4).

## Out of scope

- TUI equivalents (Phase 3); daemon-backed execution (Phase 4).

## Acceptance criteria

- [ ] A process selected into runtime managed state records membership (§28.9).
- [ ] `--dry-run` lists resolved instances + capabilities without acting.
- [ ] Partial multi-instance control returns exit 6 with per-target detail.
- [ ] Restore over drift returns exit 7 unless forced.
- [ ] `managed` shows inactive targets with last-seen (§7.4).

## Tests required

- unit: command→plan wiring; default binding modes; exit-code matrix; dry-run
  output; per-target error surfacing.
- golden: `managed` and `inspect` output (normalized).
- integration (gated): end-to-end manage→set→restore→unmanage on a helper.

## Notes

**MVP acceptance = Phases 0–2 (RFC §28).** Verify the full §28 checklist before
archiving this ticket.

## Status: DONE (2026-09-12)

Implemented in the autonomous build: control manager (capture-once original, desired/observed, drift, restore, stale-PID refusal, safeguards, stop-intent, snapshot/follow binding, inactive retention), atomic boot-scoped runtime state, and the control CLI. Verified end-to-end on a live process (nice apply/restore, with the CAP_SYS_NICE restore-privilege case surfaced correctly). Tests cover the semantics; race-clean; coverage ≥80%.
