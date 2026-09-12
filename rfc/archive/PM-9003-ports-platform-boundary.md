---
id: PM-9003
title: Establish the ports-and-adapters boundary (OS-agnostic core, pluggable backends)
state: DONE
phase: 0
depends: ["PM-0001"]
owner:
rfc: ["§24", "§25", "§32", "§14", "§15"]
---

## Summary

Bake the ports-and-adapters (hexagonal) architecture in from the start so the
core stays OS-agnostic and every extension axis — metrics, collectors, resolvers,
controllers, renderers, formats, transports, **and future operating systems
(Windows/macOS/BSD/other Unix)** — is additive. See DEVELOPMENT.md §2.4.

This is an architectural guardrail ticket: it defines the seams and the CI rules
that keep them intact, so later tickets implement *against* ports rather than
against Linux directly.

## Scope

- Create `internal/ports` holding the core-owned interfaces (illustrative names):
  `ProcessSource`, `CapabilityProbe`, `Controller`, `SignalSender`,
  `GroupController`, plus existing `Collector`/`Resolver`/`Renderer`/`Decoder`/
  `Clock`/`Transport` contracts. Small, role-specific (ISP).
- Define a startup **factory/registry** that selects the platform adapter set
  (Linux today) — no core `switch` over OS.
- Establish the build-tag convention (`*_linux.go`, `*_windows.go`, `*_darwin.go`,
  `*_bsd.go`) and the adapter package location (`internal/platform/<os>/` or the
  existing `internal/procfs` as the Linux `ProcessSource`).
- Add the **architecture guard** (CI, via PM-9001): core packages (`model`,
  `expr`, `query`, `control` planning, config-semantics, registries) may not
  import `syscall`/`x/sys` or any OS adapter. Encode the allowed dependency
  directions.
- Document the pattern + a worked "how to add a new OS backend" and "how to add a
  new metric/collector/renderer" recipe in `docs/architecture.md`.

## Out of scope

- Implementing any non-Linux adapter (future tickets); Linux adapters are built
  by their normal Phase 1/2 tickets, now *against these ports*.

## Acceptance criteria

- [ ] `internal/ports` exists; the Linux collectors/controllers implement those
      interfaces (PM-0101/0102/0203/0204 depend on / conform to them).
- [ ] No core package imports `syscall`/`x/sys`/an adapter; CI guard proves it.
- [ ] Adapter selection is registry/factory-driven, not a hardcoded switch.
- [ ] `docs/architecture.md` documents the ports, the ring diagram, and both
      "add an OS" and "add an extension" recipes.
- [ ] A throwaway spike (or documented design) shows a second, stub adapter
      registering with zero core edits.

## Tests required

- unit: registry/factory adapter selection; port contract tests reused by both
  real and fake adapters (Liskov).
- meta/CI: architecture import-guard fails on a violating branch.

## Notes

RFC §32 "Linux-only" is a **shipping-scope** decision, not an architectural one.
This ticket ensures portability is free later. If it appears to conflict with the
RFC, raise a `PM-90NN` amendment rather than deviating silently (RFC §31.10).
Deep OS/tool-specific work should conform to these seams; revisit early Phase 1
tickets to target `internal/ports` if this lands after they start.

## Status: DONE (2026-09-12)

Ports-and-adapters boundary established: internal/ports defines the core-owned interfaces; OS specifics live in internal/procfs (Linux) behind build tags; fakes in internal/testutil satisfy the same contracts. An architecture import-guard test (internal/archguard) fails the build if any core package imports syscall/x/sys or an adapter/outer package.
