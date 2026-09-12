# PROGRESS

Snapshot of what the autonomous build has implemented. Authoritative task state
lives in [`rfc/`](rfc/) (archive = done, backlog = todo); this is a human-readable
overview.

_Last updated: 2026-09-12._

## Working today

`procfit` builds and runs against live `/proc`. End-to-end observation works:

```bash
make build
./bin/procfit ps --group-by comm --leaf none --sort cpu:desc
./bin/procfit ps --group-by none --leaf process --format json
./bin/procfit ps --select 'uid == 0' --having 'cpu > 0.1' --group-by comm --leaf none
./bin/procfit stat 1s --count 3 --group-by comm --leaf none --columns target,pt,cpu
./bin/procfit metrics list --format json
./bin/procfit capabilities
./bin/procfit config check ./config.toml
./bin/procfit config convert ./config.toml --to yaml
./bin/procfit config dump --effective --format json
```

## Phase status

| Phase | Area | State |
|---|---|---|
| 0 | Module, meta, model, registries, testutil, JSON schema v1 | **done** |
| 0 | Config: canonical model, strict TOML/YAML/JSON, discovery, presets, validation, `config check/convert/dump` | **done** |
| 1 | procfs adapter (identity + parsers, fixture-testable) | **done** |
| 1 | Light-metric sampler (rates, warm-up, reset/vanish, unknown≠zero) | **done** |
| 1 | Expression DSL (`select`/`having`, type-checked) | **done** |
| 1 | Query pipeline (group/aggregate/leaves/having/sort/project) | **done** |
| 1 | Renderers: table + JSON (schema v1) | **done**; wide/csv/ndjson + golden → PM-0110 |
| 1 | `ps`, `stat`, `metrics list`, `capabilities` | **done** (stat: basic append-only) |
| 1 | Collector scheduler + multi-interval + full capability probe | todo (PM-0102) |
| 1 | Metadata resolvers (uid/app/systemd/cgroup labels) | todo (PM-0104) |
| 2 | Runtime state, managed targets, controllers, control CLI | todo (PM-02xx) |
| 3 | TUI | todo (PM-03xx) |
| 4 | Daemon + policies | todo (PM-04xx) |
| 5 | FD/cgroup/eBPF/perf/history | todo (PM-05xx) |

## Quality gates (current)

- `go test ./...` green; `go test -race ./...` green.
- Total statement coverage ≥ 80% (the enforced floor, DEVELOPMENT.md §3.2).
- `go vet` clean; `gofmt -s` clean.
- `make cover` enforces the 80% floor locally.

## Toolchain notes

The dev box had no Go/make/gcc; they were provisioned via nix
(`nix profile add nixpkgs#go nixpkgs#gnumake nixpkgs#gcc`). `go` must be on PATH
(`export PATH="$HOME/.nix-profile/bin:$PATH"`). See QUESTIONS.md.

## Known gaps / deferred

- Renderers: `wide`, `csv`, `ndjson`, and golden snapshots (PM-0110).
- Collector scheduler with independent per-collector intervals (PM-0102); today a
  single light sampler runs synchronously for `ps`/`stat`.
- `--preset`/config integration into `ps`/`stat` flag precedence (config commands
  work standalone; view presets not yet applied to live queries).
- Group-level aggregation of a permission-denied metric collapses to
  "unavailable/disabled" rather than carrying the specific reason.
