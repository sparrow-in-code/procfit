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
| 1 | Renderers: table, wide, JSON (schema v1), csv, ndjson | **done** |
| 1 | `ps`, `stat`, `metrics list`, `capabilities` | **done** |
| 1 | Metadata resolvers (uid→user, systemd-unit from cgroup) + user dimension/column | **done** |
| 1 | Collector mechanism (MetricCollector port) + FD/socket collector | **done** (PM-0102 realized via port + shared loop) |
| — | CI + golangci-lint size caps + fuzz + coverage gate + arch import-guard | **done** (PM-9001; workflow unverified w/o remote) |
| 2 | Runtime state, managed targets, nice+stop+freeze controllers, control CLI | **done** |
| 3 | TUI (headless model + tcell driver, grouping/sort/interval, prints CLI on exit) | **done** |
| 4 | Daemon: AF_UNIX IPC + peer-cred, shared engine, policy reconciliation, SIGHUP reload, systemd unit | **done** |
| 5 | FD/socket collector, cgroup v2 freeze/thaw, opt-in audit history | **done** |
| 5 | eBPF `wakeups` collector (tracepoint, waker attribution) | **done** — validated live as root |
| 5 | perf HW counters (cycles/instructions/ipc/cache-misses) | **done** (impl+validated); real values need a vPMU/bare-metal host |

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

## What's left

All planned phases (0–5) are implemented. The eBPF `wakeups` collector was
validated live as root (AlmaLinux 9, kernel 5.14); the perf collector is
implemented and validated to degrade correctly (the test KVM VM has no virtual
PMU, so hardware counter *values* require a vPMU/bare-metal host).

Smaller deferred refinements (non-blocking):

- Independent per-collector intervals in a formal scheduler (today: one shared
  loop; cost-1+ collectors gated by requested metrics).
- `--preset`/config-view integration into `ps`/`stat` flag precedence (config
  commands work standalone; live view-presets not yet applied).
- TUI managed/control panel (browser + query pickers are done).
- eBPF `timer-wakeups` and per-process `net-*` metrics (need dedicated BPF
  programs; `wakeups` is done). perf multiplexing/scaling metadata.
- Fully-normalized golden snapshot files (content/round-trip tests exist).
- Group-level aggregation of a permission-denied metric collapses to
  "unavailable/disabled" rather than the specific reason.
- Light-profile performance benchmark against the §22 budget (§28.16).
