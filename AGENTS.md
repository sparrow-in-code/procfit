# AGENTS.md — Start here

This file orients any agent (or human) working on **`pm`**, a low-overhead Linux
process observer and workload controller written in Go.

`CLAUDE.md` is a symlink to this file. Read it fully before doing anything.

---

## 1. Onboard by reading these, in order

Do not write code until you have read:

1. **[`RFC-pm-linux-process-observer-controller.md`](RFC-pm-linux-process-observer-controller.md)**
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

From RFC §6/§32 — the full list and rationale are in `DEVELOPMENT.md §2`:

1. **PID is never identity** — use `boot_id + pidns_inode + pid + starttime`;
   revalidate before every control action.
2. **Unknown is never zero** — carry availability/quality; render `-`/`?`.
3. **Renderers don't compute** — all collect/filter/group/aggregate lives in the
   engine.
4. **Control is a side plane** — never a precondition for observation.
5. **`select` (pre-group) ≠ `having` (post-aggregate).**
6. **Config: three formats, one canonical DTO, strict parsing, identical
   semantics.**
7. **Managed membership ≠ control; restore ≠ unmanage; drift is reported, not
   fought.**
8. **Never setuid; never require root by default** — degrade by capability.
9. **TDD first** — failing test, then code, then refactor. Fakes live in
   `internal/testutil`.
10. **Docs move with code** — update the relevant `.md` in the same change; JSON
    schema is a versioned public interface.

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
