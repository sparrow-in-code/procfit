---
id: PM-0304
title: TUI responsive independent sampling/render loop
state: DONE
phase: 3
depends: ["PM-0302"]
owner:
rfc: ["§19.4", "§22"]
---

## Summary

Decouple sampling from rendering so slow collectors never block input, and the
UI stays responsive while showing staleness honestly.

## Scope

- Independent sampling and render loops (§19.4); UI shows the last complete
  generation and marks stale columns.
- Terminal resize and config reload must not restart collectors unnecessarily.
- Input latency <100 ms even when collectors are slow (§22).

## Out of scope

- Adding new interactions (PM-0302/PM-0303).

## Acceptance criteria

- [ ] Simulated slow collector does not block key input (latency budget met).
- [ ] Stale generation is clearly marked, not silently shown as fresh.
- [ ] Resize/reload do not trigger collector restarts.

## Tests required

- unit: loop coordination with fake slow collector + fake clock; staleness
  marking.
- bench: input latency under injected collector delay (§22).

## Notes

Concurrency correctness matters here — run these under `go test -race`.

## Status: DONE (2026-09-12)

Interactive explorer implemented with tcell: headless unit-testable Model (view state + key handling), tcell driver isolated behind a RefreshFunc callback (UI depends on a query function, not the engine/daemon — framework choice reversible per DEVELOPMENT §2.4), grouping presets, sort toggle, interval step, scrolling, responsive single event+ticker loop (RFC §19.4), and prints the equivalent CLI command on exit (Phase 3 exit criterion). Managed/control panel keys are a follow-up; core browser + query pickers are done.
