# procfit — Linux Process Observer and Workload Controller

> `procfit` is a working name and may change (RFC §30.1); it is kept behind a single
> constant so renaming is trivial.

`procfit` is a low-overhead Linux process explorer, metrics sampler, and interactive
workload controller. It unifies the useful parts of `ps`, `pidstat`, `vmstat`,
`htop`, `powertop`, and simple cgroup/process control behind one normalized
snapshot pipeline that powers a TUI, a one-shot table, a streaming view, and
JSON/NDJSON output — plus a separate, safe control plane for managing workloads.

The default profile uses only inexpensive `/proc` and cgroup reads. It runs as
the invoking user, never setuid, and degrades by capability instead of demanding
root.

## Status

Pre-implementation. This repository currently contains the **specification and
build plan**; code is delivered ticket by ticket per the roadmap.

## Repository map

| Path | What |
|---|---|
| [`RFC-procfit-linux-process-observer-controller.md`](RFC-procfit-linux-process-observer-controller.md) | The product specification — single source of truth. |
| [`AGENTS.md`](AGENTS.md) / `CLAUDE.md` | Start-here onboarding for contributors/agents. |
| [`DEVELOPMENT.md`](DEVELOPMENT.md) | Engineering handbook: architecture, SOLID, TDD, testing, workflow, Definition of Done. |
| [`rfc/`](rfc/) | Tickets, tracked by state in `backlog/` (TODO), `wip/` (WIP), `archive/` (DONE). See [`rfc/README.md`](rfc/README.md). |
| [`docs/`](docs/) | User and architecture documentation. |

## Building & testing (once code lands)

```bash
go build ./...
go test ./...
go test -race ./...
```

## Contributing

Read [`AGENTS.md`](AGENTS.md) first. All work flows through the ticket system in
[`rfc/`](rfc/); development follows [`DEVELOPMENT.md`](DEVELOPMENT.md) (TDD, clean
architecture, docs-in-lockstep). The delivery roadmap is RFC §27, and MVP =
Phases 0–2 (RFC §28).
