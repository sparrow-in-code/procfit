---
id: PM-0301
title: TUI framework selection and rendering benchmark
state: TODO
phase: 3
depends: ["PM-0108"]
owner:
rfc: ["§19.4", "§22", "§30.4"]
---

## Summary

Decide the TUI library (Bubble Tea, tcell, or other) after a small rendering
benchmark, then stand up the app shell. Framework choice is an explicit open
question (RFC §30.4).

## Scope

- Benchmark candidate libraries rendering a ~1000-row tree at the target
  interval; measure render/input latency against §22 (<100 ms).
- Decision recorded as a PM-90NN RFC-amendment/decision note.
- Minimal TUI shell: status bar, main tree/table area, event loop skeleton
  reading from the query engine (read-only for this ticket).

## Out of scope

- Interactions/pickers (PM-0302); control (PM-0303); loop tuning (PM-0304).

## Acceptance criteria

- [ ] Benchmark exists and its numbers justify the choice in the decision note.
- [ ] Shell renders a live snapshot; `q`/`Ctrl-C` quits cleanly.
- [ ] Chosen dependency documented with rationale (DEVELOPMENT.md dep policy).

## Tests required

- unit: model/update logic decoupled from the terminal (testable without a TTY).
- bench: rendering latency harness committed.

## Notes

Keep TUI depending on the client-facing `Engine` interface (§25), never on
collector/daemon internals — so the same UI later runs against the daemon
unchanged.
