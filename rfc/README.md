# `rfc/` — Tickets and Work Tracking

This directory is the single source of truth for **what work exists and what
state it is in**. Every unit of work is a ticket: one Markdown file with a
state field in its front matter. Tickets are physically located in the
subdirectory that matches their state.

The canonical product specification lives outside this directory in
[`../RFC-procfit-linux-process-observer-controller.md`](../RFC-procfit-linux-process-observer-controller.md).
Tickets are *derived from* that spec and must cite the RFC sections they
implement. When a ticket and the RFC disagree, the RFC wins — unless the ticket
records an approved amendment (see below).

## Directory = state

| Directory | State | Meaning |
|---|---|---|
| `backlog/` | `TODO` | Accepted, not yet started. |
| `wip/` | `WIP` | Actively being implemented. **At most one owner at a time.** |
| `archive/` | `DONE` | Merged and verified against its acceptance criteria. |

The `state:` field in a ticket's front matter MUST always match the directory
it lives in. Changing state = editing the field **and** `git mv`-ing the file.

## Ticket lifecycle

```text
backlog/PM-0101-*.md   (state: TODO)
   │  agent picks it up
   ▼
git mv → wip/PM-0101-*.md   (state: WIP)   ← set your name, start TDD
   │  all acceptance criteria met, tests green, docs updated
   ▼
git mv → archive/PM-0101-*.md   (state: DONE)   ← record commit/PR + date
```

Rules:

1. Never start work without a ticket in `wip/`. If none fits, create one in
   `backlog/` first, then move it.
2. Respect `depends:` — do not start a ticket whose dependencies are not yet in
   `archive/` unless you deliberately stub the dependency and say so.
3. A ticket is only `DONE` when it satisfies the **Definition of Done** in
   [`../DEVELOPMENT.md`](../DEVELOPMENT.md) *and* its own acceptance criteria.
4. Discovered follow-up work becomes a **new** ticket in `backlog/`, not silent
   scope creep in the current one.

## Ticket ID scheme

`PM-PXNN` where `PX` encodes the phase and `NN` is a sequence number:

- `PM-00NN` — Phase 0 (skeleton & contracts)
- `PM-01NN` — Phase 1 (read-only core)
- `PM-02NN` — Phase 2 (runtime management & control)
- `PM-03NN` — Phase 3 (TUI)
- `PM-04NN` — Phase 4 (daemon & policies)
- `PM-05NN` — Phase 5 (extended collectors/controllers)
- `PM-90NN` — Cross-cutting (CI, tooling, docs, decisions)

## Ticket template

Every ticket uses this shape (see `TEMPLATE.md`):

```markdown
---
id: PM-00NN
title: Short imperative title
state: TODO            # TODO | WIP | DONE
phase: 0
depends: []            # list of ticket ids
owner:                 # set when moved to wip/
rfc: ["§X", "§Y"]      # RFC sections this implements
---

## Summary
One paragraph: what and why.

## Scope
- concrete tasks

## Out of scope
- explicit exclusions

## Acceptance criteria
- [ ] verifiable, testable statements

## Tests required
- unit / property / integration / golden / perf specifics

## Notes
Design hints, gotchas, links.
```

## Proposing changes to the spec

If implementation reveals that the RFC is wrong or underspecified, do **not**
silently deviate (RFC §31.10). Create a `PM-90NN` ticket titled
`RFC amendment: ...`, describe the change and rationale, and get it accepted
before implementing. Approved amendments are folded back into the master RFC.
