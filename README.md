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

## Installing

**Release binary** (static, Linux amd64/arm64) — from the
[GitHub Releases](https://github.com/sparrow-in-code/procfit/releases):

```bash
curl -sSL https://github.com/sparrow-in-code/procfit/releases/latest/download/procfit_<version>_linux_amd64.tar.gz | tar xz
./procfit ps
```

**Nix flake:**

```bash
nix run   github:sparrow-in-code/procfit -- ps   # run without installing
nix build github:sparrow-in-code/procfit          # -> ./result/bin/procfit
nix develop                                        # dev shell (go, make, lint, gcc, clang)
```

**Container** (GHCR, multi-arch). procfit observes the host, so share the host PID
namespace (its `/proc` then shows host processes):

```bash
docker run --rm --pid=host ghcr.io/sparrow-in-code/procfit ps
```

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

The status line stays intentionally minimal — name, `gen`/`rows`, interval, the
nav hint, and `[?]help [q]uit`. Press **`?`** for the full keymap overlay (scrolls
on a short terminal; `?`/`Esc`/`q` closes). The tables below are that keymap.

| Key | Action |
|---|---|
| `?` | toggle the in-TUI help overlay (the full keymap) |
| `↑`/`↓`, `k`/`j`, `PgUp`/`PgDn` | move the cursor |
| `Enter` / `←` / `→` | fold/unfold the group under the cursor (`[-]` open, `[+]` collapsed; leaves have no marker) |
| `c` / `C` | fold **all** groups shut / unfold **all** at once (a per-group `Enter` still overrides afterwards) |
| `i` | inspect the selected row — identity + recent control history (when `[state].history` is enabled); `Esc`/`i` to close |
| `g` | cycle grouping — the built-in presets **plus any displayed column that is a dimension** (e.g. `wchan`, `pstate`, `user`, `hostname`), so you can group by what's on screen |
| `s` / `S` | cycle the sort column / toggle ascending↔descending |
| `/` or `f` | filter (see below) |
| `+` | **column picker** — a type-to-filter list of every column with its description; `↑↓` move, `Enter` toggles a column on/off (stays open to enable several), `Esc` closes. No restart |
| `-` | quick-drop the rightmost column |
| `u` | toggle human units (K/M/G) vs raw bytes |
| `[` / `]` | faster / slower refresh interval (start value via `--interval`, e.g. `procfit tui --interval 2s`) |
| `p` or `Space` | pause/resume auto-refresh (freeze the view to aim at a moving target) |
| `r` | refresh once |
| `q` / `Ctrl-C` | quit (prints the equivalent `procfit ps …` command for the current view) |

**Throttling / control** (on the selected process *or* a whole group — a group
action targets every process under it; every action shows a preview with the
affected count and requires `y` to confirm):

| Key | Action |
|---|---|
| `n` / `N` | renice (type `-20..19`) / restore original nice |
| `x` / `X` | stop (SIGSTOP) / continue (SIGCONT) |
| `z` / `Z` | freeze / thaw the whole **cgroup subtree** (v2, if delegated) — see the safety note |

Control keys follow one convention: **lowercase applies/enforces, uppercase lifts
it** (`n`/`N`, `x`/`X`, `z`/`Z`). Every action shows a preview and needs `y`. The
action is **pinned to the process(es) selected when you pressed the key**, and the
view **freezes while the confirmation gate is open**, so a background refresh
re-sorting the list can never retarget your action — confirming always hits the
process you previewed, not whatever row drifted under the cursor.

> **Freeze safety.** `x` stop is per-process (SIGSTOP); `z` **freeze acts on the
> whole cgroup subtree**. procfit refuses to freeze a cgroup that is your login
> session / user slice or an ancestor of procfit itself (it would lock you out) —
> the TUI shows `⚠ REFUSED` in the forecast and skips it. To override from the CLI:
> `procfit set <target> --freeze-cgroup --force`.
>
> **`--dry-run` never acts.** Every mutating control op honours it uniformly —
> `set --nice/--stop/--continue/--freeze`, `signal`, and `restore` all report the
> would-be per-pid outcome (`dry-run: would …`) without touching a process,
> recording state, or writing an audit event. A destructive `signal --dry-run`
> needs no `--yes`, since nothing is delivered.

Control reuses the same safeguards as the CLI `set`/`restore` commands (protected
PIDs are skipped, changes are recorded for restore). In the confirmation gate,
press `v` to view the full per-process forecast — each pid's current→desired
value, which are protected/skipped, and a warning when raising priority (lower
nice) needs privilege and may not be restorable.

When `[state].history` is enabled, control actions (from the CLI `set`/`restore`
or the TUI) are recorded to an opt-in audit log; `procfit history [--pid N]`
prints recent events, and the detail overlay (`i`) shows them per process.

Press **`Tab`** to switch to the **managed panel** — the targets you've controlled
(named e.g. `idea#281839`). It renders with the *same* grouped tree as the browser,
resolving each managed pid to its live process **by identity, not pid** — a pid
whose process has exited (or been reused by an unrelated process with a different
start time) moves under a built-in **orphans** group, still labelled by the name
it had while alive, and is never controlled as the wrong process. The same control
keys work here, acting on the *retained target* (so continuing/thawing/renicing
what you paused is right here):

| Key | Action (managed panel) |
|---|---|
| `n` / `N` | renice / restore original nice on the selected target |
| `x` / `X` | stop / continue |
| `z` / `Z` | freeze / thaw |
| `d` | **drop** — stop tracking the target (does **not** revert; lift first with `N`/`X`/`Z` to undo) |
| `r` / `Tab` / `q` | refresh / back to browser / quit |

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

- **One word (no spaces) = a case-insensitive target substring.** Typing `idea`
  matches any target containing "idea"; `Enter` applies it. As you type, matching
  **column names are suggested** as hints (that word may still just be a substring).
- **Add a space and you're writing a column filter** — the first word is taken as
  a column, and an **operator autosuggest** appears (type-aware: comparisons for
  numbers, `==`/`~=`/`contains` for strings). Here `Enter` (or `Tab`) **autofills**
  the first suggestion; the value you type after it has no autocomplete.
- **`Tab`** completes the highlighted suggestion in any position (even the first
  word, to pick a column). After **`&&`** / **`||`** it's clearly a query, so the
  next column also autofills on `Enter`.
- Line editing: `←`/`→`/`Home`/`End` move, `Ctrl+←`/`Ctrl+→` jump by word,
  `Backspace`/`Delete` edit, `↑`/`↓` recall previous filters, `Esc` cancels; an
  empty filter clears. Only the **displayed** columns may be referenced.

## Finding battery drain

CPU% alone misses the usual culprit — a low-CPU process that wakes the CPU out of
idle thousands of times a second. Two profiles answer two questions:

```bash
procfit ps --profile power                                # how much is drawn: real watts + deep-idle
procfit ps --profile battery --sort wakeups:desc          # what causes it: wakeups (needs root/eBPF)
procfit ps --profile battery --sort ctxsw-voluntary:desc  # no-root proxy for wake/sleep churn
```

`--profile power` shows **actual watts** — `power-pkg`/`power-core`/`power-dram`
from Intel/AMD RAPL (via the powercap tree) or hwmon sensors, and `power-system`
(whole-machine draw) from the laptop battery, which is vendor- and arch-neutral
(works on any x86/ARM laptop on discharge). procfit picks the first available
backend — see `procfit capabilities` for which — and a domain the host does not
expose renders unavailable, never a fake zero. Whole-machine readings (power,
C-state, PSI) are **host-scoped**: they are read once (not per process) and shown
in a **system section** above the process table — a banner line, a blank line, an
aligned `label=value` matrix, a blank line, then the process table — so they never
crowd the per-process columns. In JSON they appear under a top-level `host` object.
`wakeups` also now has a **no-root fallback** from `/proc/PID/sched` (needs
`CONFIG_SCHEDSTATS`); eBPF is used when available (richer waker attribution).

A **profile** is a named metric set *and* a default view: it can predefine the
columns (and their order), the sort, grouping, and a filter, all applied unless you
override them. So `--profile battery` pairs the whole-system draw (`power-system`
watts) and deep-idle context (`cstate-deep-residency`) with the per-process drain
drivers (`cpu`, `wakeups`/`timer-wakeups`/`ctxsw-voluntary`, `gpu`/`gpu-mem`) without
`--columns`, `--profile memory` sorts by
`pss`, and `--profile sysload` sorts by `runq-delay` with a `pstate`/`wchan` view —
any explicit `--columns`/`--sort`/`--group-by`/`--having` still wins. Without
privilege, `wakeups`/`timer-wakeups` render as unavailable and `ctxsw-voluntary`
carries the signal.

> The flag is **`--profile`** (the older `--metrics` still works as a deprecated
> alias; env `PROCFIT_PROFILE`, config `[metrics].profile`).

Run **`procfit metrics`** (`--format json` for machine output) for the full field
catalog — one list of every id usable in `--columns`/`--group-by`/`--sort`/`--having`,
each with a **USE** marker (`c`=column `g`=group-by `s`=sort `f`=filter), its unit/cost
(for metrics), and a description. It merges the metric, column, and dimension
registries by id, so it never drifts from what the tool actually accepts:

```
ID       USE   UNIT             COST  DESCRIPTION
cpu      c-sf  percent_one_cpu  0     CPU usage as percent of one CPU …
wchan    cgsf  -                -     kernel symbol a blocked task sleeps in …
cgroup   -g-f  -                -     control-group path (cgroup v2)
```

**GPU** is another invisible drain. `gpu` (per-process engine utilization %) and
`gpu-mem` come from the vendor-neutral DRM fdinfo ABI (`/proc/PID/fdinfo`), so one
path covers Intel (i915/xe), AMD (amdgpu), and ARM DRM drivers — no vendor tools:

```bash
procfit ps --profile power --sort gpu:desc   # power profile adds gpu + gpu-mem
procfit ps --columns target,pid,gpu,gpu-mem --sort gpu:desc
```

`gpu` sums a process's busy time across GPU engines over a short window (it can
exceed 100% when several engines run at once, like CPU% across cores). For the
per-engine split, add `gpu-render`, `gpu-compute`, `gpu-copy`, `gpu-video`, and
`gpu-video-enhance` — each driver's engine names are mapped onto these standard
classes, and cycles-based drivers (`drm-cycles`/`drm-maxfreq`, e.g. v3d) are
handled too. A process with no GPU client shows `-` (never a fake `0`); another
user's processes show `?` until you have privilege. Proprietary **NVIDIA** does
not expose the standard fdinfo, so its clients are invisible here unless built
with the optional NVML adapter (PM-0522); nouveau works via DRM.

## Troubleshooting load average

Load average counts tasks that are **Runnable (R)** *or* in **Uninterruptible
sleep (D)** — so high load can be CPU contention *or* I/O/kernel blocking, and CPU%
alone won't tell you which. The `sysload` profile decomposes it, and ships a default
view (columns incl. `pstate` + `wchan`, sorted by `runq-delay`):

```bash
procfit ps --profile sysload                          # the default load view
procfit ps --profile sysload --sort blkio-delay:desc  # I/O-blocked tasks (D side)
procfit ps --profile sysload --select 'state == "D"'  # only the uninterruptible tasks
procfit ps --group-by wchan --select 'state == "D"' --columns target,procs,wchan
```

`runq-delay` is the % of wall time a task was runnable but waiting for a CPU (the
R/CPU-contention signal, from `/proc/PID/schedstat`; needs `CONFIG_SCHEDSTATS`).
`blkio-delay` is the % of time blocked on block I/O (the D/I/O signal, from
`/proc/PID/stat`; needs kernel delay accounting — otherwise it reads unavailable,
not a false `0`). Alongside them the profile shows `cpu`, `ctxsw-involuntary`
(preemption churn), and `major-faults` (page-in thrash).

The **`wchan`** column is the key to the *sneaky* culprits: it's the kernel function
a blocked task is sleeping in, which names the cause `blkio-delay` alone can't —
`jbd2_log_wait_commit` (filesystem journaling), `rpc_wait_bit_killable` (NFS),
`raid5_get_active_stripe` (RAID), `__alloc_pages_slowpath` (memory reclaim),
`rwsem_down_*` (lock contention). **Group by `wchan`** to cluster every task stuck in
the same place — instant root cause. It reads `/proc/PID/wchan` only when a query
references it, and is filterable (`--select 'wchan ~= "jbd2"'`).

The **`pressure` profile** adds the direct "why is load high" readout: host PSI
(`psi-cpu`/`psi-io`/`psi-mem` — % of time some task stalled on CPU/I/O/memory, from
`/proc/pressure/*`, needs `CONFIG_PSI`) alongside each process's own R/D thread
counts (`threads-running`, `threads-uninterruptible` — its runnable and I/O-wait
contribution to load):

```bash
procfit ps --profile pressure   # PSI + per-process R/D thread counts
```

> Full per-task **delay accounting** (taskstats/netlink: reclaim/swap-in/thrash
> delays, needs `CAP_NET_ADMIN`) remains a follow-up (PM-0521); the schedstat/stat
> proxies (`runq-delay`/`blkio-delay`) and PSI cover the common cases without it.

## Seeing real memory use

RSS double-counts shared libraries across processes; the `memory` profile shows the
honest footprint:

```bash
procfit ps --profile memory --sort pss:desc
```

`pss` (proportional set size) splits shared memory fairly between its users, `uss`
(unique set size) is the private memory that would be freed if the process died —
both from `/proc/PID/smaps_rollup` (another user's processes show `?` until you have
privilege). The profile also breaks resident memory into `rss-anon`/`rss-file`
/`rss-shmem`, and shows `swap` (swapped-out), `mem-peak` (VmHWM), and `oom-score`
(0–1000 kill-likelihood — what the kernel sacrifices first under pressure).

## Per-process sockets

The `network` profile shows what each process holds on the network — no eBPF, no
root (for your own processes):

```bash
procfit ps --profile network --sort sock-tcp:desc
```

`sock-tcp`/`sock-udp`/`sock-unix` count sockets by protocol; `sock-listen`
(server side), `sock-estab`, `sock-timewait`, and `sock-closewait` break TCP down
by state. They're computed by joining each process's socket fds
(`/proc/PID/fd → socket:[inode]`) with `/proc/net/{tcp,udp,unix}`, so a leaked
listener or a pile of `TIME_WAIT` is obvious at a glance. The eBPF `net-*-bps`
/`net-*-pps` throughput metrics stay the (privileged) layer on top; sockets in a
different network namespace or of other families simply aren't counted (never a
fabricated zero). Another user's sockets need privilege.

## Grouping by container (poor-man's)

The `hostname` field is the process's `HOSTNAME` environment variable — which
container runtimes set to the container id/name — falling back to the host's own
hostname for everything not in a container. So a quick per-container view needs no
runtime integration:

```bash
procfit ps --group-by hostname --sort cpu:desc
```

It reads `/proc/PID/environ` only when a query references `hostname` (and another
user's environ shows the host fallback until you have privilege). It is also a
column, sortable, and filterable (`--select 'hostname ~= "web-"'`). For true
cgroup/runtime-derived identity, the `container`/`pod`/`cgroup` dimensions remain.

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
collapse-groups false             default  PROCFIT_COLLAPSE_GROUPS
...
```

`collapse-groups` (config key `collapse_groups`, flag `--collapse-groups`, env
`PROCFIT_COLLAPSE_GROUPS`) starts the TUI with every group folded shut — handy for
a large grouped tree you want to open selectively; `c`/`C` toggle all groups live.
It is a TUI view knob; `ps`/`stat` accept it but ignore it.

`procfit config dump --effective` prints the normalized config file itself
(defaults + file), while `--show-config` reflects a specific invocation including
env and args.

## Contributing

Read [`AGENTS.md`](AGENTS.md) first. All work flows through the ticket system in
[`rfc/`](rfc/); development follows [`DEVELOPMENT.md`](DEVELOPMENT.md) (TDD, clean
architecture, docs-in-lockstep). The delivery roadmap is RFC §27, and MVP =
Phases 0–2 (RFC §28).
