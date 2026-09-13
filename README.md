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

Under active construction; the **MVP (Phases 0–2) is substantially complete**.
`procfit ps`/`stat` observe live `/proc` with grouping, aggregation, the
`select`/`having` expression DSL, and table/JSON output; `config
check/convert/dump`, `metrics`, and `capabilities` work; and the control plane —
`manage`/`set`/`restore`/`unmanage`/`signal` with runtime managed targets, nice
and stop/continue control, original/desired/observed tracking, drift detection,
and atomic boot-scoped state — is implemented and tested. The interactive TUI
(Phase 3), the daemon (Phase 4), and the extended collectors — FD/socket, cgroup
v2 freeze/thaw, eBPF wakeups, perf counters, audit history (Phase 5) — are now
implemented as well. See [`PROGRESS.md`](PROGRESS.md) for live status and
[`rfc/`](rfc/) for tasks.

## Repository map

| Path | What |
|---|---|
| [`RFC-procfit-linux-process-observer-controller.md`](RFC-procfit-linux-process-observer-controller.md) | The product specification — single source of truth. |
| [`AGENTS.md`](AGENTS.md) / `CLAUDE.md` | Start-here onboarding for contributors/agents. |
| [`DEVELOPMENT.md`](DEVELOPMENT.md) | Engineering handbook: architecture, SOLID, TDD, testing, workflow, Definition of Done. |
| [`rfc/`](rfc/) | Tickets, tracked by state in `backlog/` (TODO), `wip/` (WIP), `archive/` (DONE). See [`rfc/README.md`](rfc/README.md). |
| [`docs/`](docs/) | User and architecture documentation. |

## Building & testing

Requires Go (1.26+). The Makefile wraps the common tasks:

```bash
make build      # -> ./bin/procfit
make test       # go test ./...
make race       # go test -race ./...
make cover      # coverage with the >=80% gate
make check      # full local gate (fmt, vet, lint, test, race, cover)
./bin/procfit ps --group-by comm --leaf none --sort cpu:desc
```

## Interactive explorer (TUI)

Run `procfit` on a TTY (or `procfit tui`) to launch the interactive explorer. It
shows the same grouped tree as `ps`, refreshed on an interval.

| Key | Action |
|---|---|
| `↑`/`↓`, `k`/`j`, `PgUp`/`PgDn` | move the cursor |
| `Enter` / `←` / `→` | fold/unfold the group under the cursor (`[-]` open, `[+]` collapsed; leaves have no marker) |
| `g` | cycle the grouping preset |
| `s` / `S` | cycle the sort column / toggle ascending↔descending |
| `/` or `f` | filter (see below) |
| `u` | toggle human units (K/M/G) vs raw bytes |
| `[` / `]` | slower / faster refresh interval |
| `p` or `Space` | pause/resume auto-refresh (freeze the view to aim at a moving target) |
| `r` | refresh once |
| `q` / `Ctrl-C` | quit (prints the equivalent `procfit ps …` command for the current view) |

**Throttling / control** (on the selected process *or* a whole group — a group
action targets every process under it; every action shows a preview with the
affected count and requires `y` to confirm):

| Key | Action |
|---|---|
| `n` | renice (type a value `-20..19`, then confirm) |
| `x` / `c` | stop (SIGSTOP) / continue (SIGCONT) |
| `z` / `Z` | freeze / thaw (cgroup v2, if delegated) |
| `R` | restore captured original values |

Control reuses the same safeguards as the CLI `set`/`restore` commands (protected
PIDs are skipped, changes are recorded for restore).

Press **`Tab`** to switch to the **managed panel** — the targets you've controlled,
with their mode, active state, member count and desired nice/stop/freeze. There,
`R` restores the selected target to its captured originals and `d` unmanages it;
`Tab` returns to the browser.

Group rows are **aggregates** of their members; the `PID`/`TID` columns are blank
on groups and populated only on process/thread leaves, so a single process is
never mistaken for a group. Sorting and filtering **preserve the tree**: siblings
are ordered within each level, and a branch is kept when the group *or any
descendant* matches.

## Filtering (the `having` expression language)

Both the TUI filter and `ps --having` accept the same expression language. Field
names are the lowercase column ids (e.g. `cpu`, `rss`, `name`, `target`).

- Comparisons: `cpu > 5`, `rss > 500000000`, `threads >= 10`
- Strings: `name == "bash"`, `target contains "chrome"`, `comm ~= "^chr"`,
  `comm in ["bash", "zsh"]`. String quotes may be `"` or `'`.
- Chain clauses with `&&` / `||` and group with parentheses:
  `cpu > 2 && (rss > 1G || threads > 100)`
- Size and duration literals: `rss > 1G`, `age > 10m`

In the TUI filter line:

- A **bare word** with no operators is shorthand for a target substring match —
  typing `idea` means `target contains "idea"`.
- Line editing: `←`/`→`/`Home`/`End` move, `Ctrl+←`/`Ctrl+→` jump by word,
  `Backspace`/`Delete` edit, `↑`/`↓` recall previous filters, `Enter` applies,
  `Esc` cancels; an empty filter clears. Only the **displayed** columns may be
  referenced (the prompt lists them).

## Configuration & precedence

View settings (grouping, columns, sort, format, target width, …) can come from
several layers. Each setting is resolved independently across a precedence order,
lowest to highest:

```
default  <  config file  <  environment  <  CLI args
```

- **CLI args** always win by default (`--group-by comm`).
- **Environment** variables are `PROCFIT_<SETTING>` with `-`→`_`, e.g.
  `PROCFIT_GROUP_BY=comm`, `PROCFIT_TARGET_WIDTH=80`.
- **Config file** is discovered (`--config`, `$PROCFIT_CONFIG`, or XDG) unless
  `--no-config` is given.

The order is reorderable per invocation with `--config-precedence` (comma list of
`config,env,args`; `default` is always the floor). For example, to let the
environment override CLI args: `--config-precedence config,args,env`.

Inspect the fully-resolved settings and **where each value came from** with
`--show-config`:

```console
$ PROCFIT_LEAF=thread procfit ps --group-by comm --show-config
SETTING        VALUE              SOURCE   ENV
group-by       comm               args     PROCFIT_GROUP_BY
leaf           thread             env      PROCFIT_LEAF
target-width   0                  default  PROCFIT_TARGET_WIDTH
...
```

`procfit config dump --effective` prints the normalized config file itself
(defaults + file), while `--show-config` reflects a specific invocation including
env and args.

## Contributing

Read [`AGENTS.md`](AGENTS.md) first. All work flows through the ticket system in
[`rfc/`](rfc/); development follows [`DEVELOPMENT.md`](DEVELOPMENT.md) (TDD, clean
architecture, docs-in-lockstep). The delivery roadmap is RFC §27, and MVP =
Phases 0–2 (RFC §28).
