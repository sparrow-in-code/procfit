# AGENTS.md — Start here

This file orients any agent (or human) working on **`procfit`**, a low-overhead Linux
process observer and workload controller written in Go.

`CLAUDE.md` is a symlink to this file. Read it fully before doing anything.

---

## 1. Onboard by reading these, in order

Do not write code until you have read:

1. **[`RFC-procfit-linux-process-observer-controller.md`](RFC-procfit-linux-process-observer-controller.md)**
   — the product specification and single source of truth. At minimum internalize
   §6 (principles/invariants), §27 (delivery plan), §28 (MVP acceptance), and
   §32 (normative decisions).
2. **[`DEVELOPMENT.md`](DEVELOPMENT.md)** — architecture, SOLID application,
   clean-code rules, the TDD/testing discipline, repo layout, workflow, and the
   Definition of Done.
3. **[`rfc/README.md`](rfc/README.md)** — how tickets and work states work.
4. **The ticket currently in [`rfc/wip/`](rfc/wip/)** — your active task (if
   any). If `wip/` is empty, pick the next unblocked ticket from
   [`rfc/backlog/`](rfc/backlog/).
5. Skim [`docs/`](docs/) for anything relevant to your ticket.

If you skip the RFC/DEVELOPMENT reading you *will* violate an invariant. Don't.

---

## 2. How work is tracked

Tickets are Markdown files whose **directory equals their state**:

- `rfc/backlog/` → `TODO`
- `rfc/wip/` → `WIP` (one per owner at a time)
- `rfc/archive/` → `DONE`

Changing state = editing the `state:` front-matter field **and** `git mv`-ing the
file to the matching directory. Full rules and the ticket template are in
[`rfc/README.md`](rfc/README.md).

**Loop:** pick unblocked ticket → `git mv` to `wip/` (set `state: WIP`, `owner`)
→ TDD it → keep docs current → run the full gate → `git mv` to `archive/`
(`state: DONE`, record commit + date) → spin off any follow-ups as new
`backlog/` tickets.

Respect `depends:` — do not start a ticket whose dependencies are not yet in
`archive/`. Build in RFC phase order (§27) and keep every phase runnable.

---

## 3. Golden rules (violating these is a bug, not a style nit)

The project owner stresses these hardest — **SOLID, clean code, ≥80% coverage,
capped unit sizes, and pervasive extensibility (including future OS backends)**.
Details in `DEVELOPMENT.md §0` (prime directives), §2 (architecture), §3
(testing). Domain invariants come from RFC §6/§32.

**How we build (enforced in CI):**

1. **SOLID, always** — one reason to change per type; extension points are
   interfaces; depend on abstractions. Adding a variant = *adding a type +
   registering it*, never editing a `switch`. (`DEVELOPMENT.md §2.2–2.3`)
2. **Extensible & portable by default** — ports-and-adapters core (§2.4). The
   core imports nothing OS-specific; new metrics/collectors/renderers/formats/
   transports **and new OSes (Windows/macOS/BSD)** are additive adapters. If a
   change would need a core edit or a central switch to support a variant, redesign.
3. **Clean code, capped size** — honour the hard caps in `DEVELOPMENT.md §2.6`
   (func ≤60 lines, file ≤600, ≤6 interface methods, etc.). Big units = split.
4. **≥ 80% coverage, TDD first** — failing test, then code, then refactor; 80% is
   a hard CI gate (§3.2). Fakes live in `internal/testutil`.
5. **Docs move with code** — update the relevant `.md` in the same change; JSON
   schema is a versioned public interface.

**Domain invariants (RFC §6/§32):**

6. **PID is never identity** — use `boot_id + pidns_inode + pid + starttime`;
   revalidate before every control action.
7. **Unknown is never zero** — carry availability/quality; render `-`/`?`.
8. **Renderers don't compute; control is a side plane** — all
   collect/filter/group/aggregate lives in the engine; observation works without
   control.
9. **`select` (pre-group) ≠ `having` (post-aggregate); config = 3 formats, one
   canonical DTO, strict, identical semantics.**
10. **Managed membership ≠ control; restore ≠ unmanage; drift is reported, not
    fought. Never setuid; never require root** — degrade by capability.

---

## 4. Practical commands

```bash
go build ./...
go test ./...
go test -race ./...
go vet ./... && staticcheck ./...
go test -tags integration ./...    # gated; needs a disposable ns/container
```

A change is done only when it meets the **Definition of Done** in
`DEVELOPMENT.md §7` and its ticket's acceptance criteria.

---

## 5. When the spec is wrong or unclear

Do **not** silently deviate (RFC §31.10). Open a `PM-90NN` ticket titled
`RFC amendment: ...`, state the change and rationale, get it accepted, then fold
it back into the master RFC. Present deviations as proposed amendments.
