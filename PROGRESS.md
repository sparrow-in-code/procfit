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

# control plane (Phase 2) — runtime managed targets + nice/stop control
./bin/procfit manage pid:1234 --name chrome --nice 10 --set-nice
./bin/procfit managed
./bin/procfit set managed:chrome --stop
./bin/procfit restore managed:chrome        # refuses on external drift (exit 7)
./bin/procfit signal TERM pid:1234 --yes
./bin/procfit unmanage managed:chrome
```

## MVP acceptance (RFC §28)

Phases 0–2 are the MVP. ~15 of 17 §28 criteria are met and test-covered
(grouping/aggregation, flat list, streaming, cross-format config equivalence &
strictness, profiles + overrides, select/having, PID+starttime identity with
stale-action refusal, managed membership, nice original/desired/observed +
restore, external-drift detection, stop-intent vs observed PSTATE, inactive
follow targets, private/atomic/boot-scoped state, unknown≠zero, versioned JSON
schema). Remaining: the light-profile performance benchmark (§28.16) and full
golden snapshots (§28.17 / PM-0110).

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
| 1 | Metadata resolvers (uid→user, systemd-unit from cgroup) + user dimension/column | **done** |
| 1 | Collector scheduler + multi-interval + full capability probe | todo (PM-0102) |
| — | CI (GitHub Actions) + golangci-lint size caps + fuzz targets + coverage gate | **done** (PM-9001; workflow unverified w/o remote) |
| 2 | Runtime state (atomic, boot-scoped, 0600/0700, quarantine) | **done** |
| 2 | Managed targets (snapshot/follow, inactive retention) | **done** |
| 2 | Nice + stop/cont controllers (original/desired/observed, drift, restore, stale-refusal, safeguards) | **done** |
| 2 | Control CLI (manage/set/restore/unmanage/signal/managed) + target resolver | **done** |
| 3 | TUI | todo (PM-03xx) |
| 4 | Daemon + policies | todo (PM-04xx) |
| 5 | FD/cgroup/eBPF/perf/history | todo (PM-05xx) |

## Quality gates (current)

`make check` runs the whole gate and passes:

- `go test ./...` green; `go test -race ./...` green.
- Total statement coverage **81.5% ≥ 80%** (enforced floor, DEVELOPMENT.md §3.2).
- `go vet` clean; `gofmt -s` clean.
- `golangci-lint run` — **0 issues**, including the §2.6 size-cap linters
  (funlen/gocyclo/gocognit/nestif) and staticcheck.
- Fuzz targets (procfs stat/io parsers, expr compiler) run crash-free
  (`./scripts/fuzz-smoke.sh`).

## Toolchain notes

The dev box had no Go/make/gcc; they were provisioned via nix
(`nix profile add nixpkgs#go nixpkgs#gnumake nixpkgs#gcc`). `go` must be on PATH
(`export PATH="$HOME/.nix-profile/bin:$PATH"`). See QUESTIONS.md.

## What's next (largest remaining)

- **Phase 3 — TUI** (PM-0301..0304): needs a framework choice + rendering
  benchmark, then the interactive browser/pickers/control panel.
- **Phase 4 — daemon** (PM-0401..0404): AF_UNIX protocol, shared engine,
  continuous policy reconciliation, user systemd unit + reload.
- **Phase 5 — extended collectors** (PM-0501..0505): FD/socket, cgroup v2
  freeze/CPU/IO, eBPF, perf, history.

## Known gaps / deferred

- Renderers: `wide`, `csv`, `ndjson`, and golden snapshots (PM-0110).
- Collector scheduler with independent per-collector intervals (PM-0102); today a
  single light sampler runs synchronously for `ps`/`stat`.
- `--preset`/config integration into `ps`/`stat` flag precedence (config commands
  work standalone; view presets not yet applied to live queries).
- Group-level aggregation of a permission-denied metric collapses to
  "unavailable/disabled" rather than carrying the specific reason.
- Light-profile performance benchmark against the §22 budget (§28.16).
