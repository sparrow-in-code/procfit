# RFC: `pm` — Linux Process Observer and Workload Controller

| Field | Value |
|---|---|
| Status | Draft for implementation |
| Revision | 0.1 |
| Date | 2026-09-12 |
| Target platform | Linux |
| Reference implementation | Go |
| Working binary name | `pm` |
| Primary audience | Implementers, reviewers, and early users |

> `pm` is a working name. The implementation must keep the command name easy to
> change because `pm` is generic and may collide with an existing executable.

## 1. Abstract

`pm` is a low-overhead Linux process explorer, metrics sampler, and interactive
workload controller. It combines the useful parts of `ps`, `pidstat`, `vmstat`,
`htop`, `powertop`, and simple cgroup/process-control tooling while keeping the
following concepts independent:

1. what entities and metrics are collected;
2. which entities are selected;
3. how entities are grouped;
4. whether leaves are processes, threads, or omitted;
5. how metrics are aggregated, filtered, sorted, and rendered;
6. which logical targets are managed;
7. which changes `pm` applied, what the original values were, and whether the
   live system has drifted.

The same normalized snapshot pipeline powers:

- a full-screen TUI;
- a one-shot `ps`-like table;
- a repeated `vmstat`-like stream;
- JSON/NDJSON output;
- a managed-workload view;
- control and restore commands.

The default profile uses only inexpensive `/proc` and cgroup reads. More costly
features such as FD classification, per-process network byte attribution,
scheduler wakeups, syscall activity, or hardware counters are capability-gated
collectors with independent sample intervals.

## 2. Motivation

Existing tools expose different slices of the same problem:

- `ps` and `top` primarily show current processes;
- `pidstat` shows process rates but has limited grouping and control state;
- `htop` is interactive but is centered on a process tree;
- `powertop` focuses on power and wakeups;
- `perf` and eBPF tools are powerful but expensive and task-specific;
- cgroup tools control workloads but do not preserve user-facing original,
  desired, and observed state.

The motivating use case is finding and suppressing workloads that waste battery
or cause performance instability. The design is intentionally broader: the same
engine should also work as a process/namespace explorer, I/O inspector, process
churn monitor, and interactive scheduler controller.

The central product feature is not any single metric. It is the ability to say:

```text
observe these entities
→ select these records
→ group by these dimensions in this order
→ aggregate these metrics
→ keep these leaves, or no leaves
→ filter and sort the rows
→ render them in the desired mode
```

while a separate state manager can say:

```text
this logical target is managed
→ these concrete process instances currently match it
→ this is what pm changed
→ this is the captured original value
→ this is the desired value
→ this is what the kernel currently reports
```

## 3. Goals

### 3.1 Functional goals

- Enumerate Linux processes and optionally individual threads.
- Normalize process, thread, namespace, cgroup, session, application, and
  executable identity into a stable internal model.
- Support ordered, arbitrary grouping pipelines such as:
  `namespace-set,app,session,process,thread`.
- Allow grouping to be disabled entirely.
- Let the final leaf level be `process`, `thread`, or `none`, independently of
  grouping.
- Collect inexpensive CPU, memory, process-state, fault, context-switch, block
  I/O, and churn metrics by default.
- Add expensive metrics as independent collectors and metric capabilities.
- Provide one-shot, streaming, TUI, JSON, and NDJSON renderers.
- Provide an expression language for entity selection and aggregate filtering.
- Provide multi-column sorting.
- Persist custom presets and managed selectors in TOML, YAML, or JSON.
- Track runtime manual management and every `pm`-applied control change.
- Preserve original, desired, and observed values for safe restore and drift
  detection.
- Allow a managed logical target to remain visible while it has zero matching
  processes.
- Make refresh intervals configurable globally and per collector.
- Be useful without root; degrade by capability when permissions are missing.
- Expose machine-readable capability and metric metadata.

### 3.2 Quality goals

- Default idle overhead must be low enough for continuous laptop use.
- A stale PID must never cause a control action to be applied to a reused PID.
- Configuration must be strict: unknown keys, duplicate keys, bad enum values,
  invalid durations, and invalid selectors are errors.
- Missing kernel capabilities or permissions must produce explicit unavailable
  values, not zeroes.
- The core query model must be renderer-independent.
- The control layer must be collector- and renderer-independent.
- The architecture must permit procfs, cgroup, eBPF, netlink, and perf-backed
  collectors without coupling them to the UI.

## 4. Non-goals

The first stable release does not aim to:

- replace a system service manager such as systemd;
- provide distributed host monitoring;
- offer a web UI or remote network API;
- persist long-term time-series data;
- estimate watts or battery lifetime with scientific accuracy;
- assign per-process Linux CPU `iowait`, which the kernel does not expose as a
  meaningful per-process metric;
- make eBPF mandatory;
- use a setuid executable;
- promise atomic all-or-nothing control of a changing multi-process group;
- infer friendly names for every Linux namespace. Native namespace identity is
  an inode, not a human-readable name;
- automatically control system-critical processes unless the user explicitly
  opts into an unsafe override.

## 5. Terminology

| Term | Meaning |
|---|---|
| Entity | A normalized observed process or thread record. |
| Process instance | A PID plus its kernel start time; not merely a numeric PID. |
| Thread instance | A `(tgid, tid)` pair plus start time. |
| Dimension | A field usable as one stage of grouping, for example `app` or `pidns`. |
| Group | A row aggregating entities with the same dimension key at one grouping stage. |
| Leaf | The optional terminal process or thread rows beneath groups. |
| Logical target | A persistent identity or selector that may bind to zero or more live instances. |
| Binding | The current mapping from a logical target to concrete process/thread instances. |
| Managed membership | The fact that a target is retained in the managed view. It does not imply a control action. |
| Policy | Persistent user intent from configuration. |
| Manual target | A runtime-only target created interactively or from CLI. |
| Observed state | What the kernel reports now. |
| Desired state | What `pm` intends a controlled field to be. |
| Original state | The value captured before `pm` first changed that field for that instance. |
| Drift | Observed state differs from desired state after `pm` applied or reconciled it. |
| Restore | Attempt to return fields to their captured original values. |
| Unmanage | Stop tracking/enforcing a target; does not imply restore. |
| Collector | A component producing entity attributes or metrics. |
| Capability | A collector/backend feature available under the current kernel and privileges. |

## 6. Design principles and invariants

### 6.1 Independent query stages

The observation path is:

```text
collectors
  → normalized entities and samples
  → entity selector
  → grouping pipeline
  → aggregation
  → optional leaves
  → aggregate/row filter (having)
  → multi-key sort
  → column projection
  → renderer
```

No renderer may implement its own collection, filtering, grouping, or aggregate
semantics.

### 6.2 Control is a side plane

Control must not be embedded in the query pipeline:

```text
logical target or snapshot selection
  → resolve current bindings
  → validate process instance identity
  → controller backend
  → record original/desired/observed state
  → reconcile and detect drift
```

Query rows may be annotated from control state, but observation continues to
work when control is unavailable.

### 6.3 Numeric PID is never identity

The canonical process identity is:

```text
boot_id + pid_namespace_inode + pid + starttime_ticks
```

At minimum, local operations must compare `pid` and field 22 (`starttime`) from
`/proc/<pid>/stat`. `boot_id` is read from
`/proc/sys/kernel/random/boot_id`. A controller must revalidate identity
immediately before acting. Where supported, signal delivery should use pidfds.

### 6.4 Unknown is not zero

Every metric sample has availability and quality metadata. Examples:

- permission denied;
- kernel feature disabled;
- first sample has no rate yet;
- process vanished while reading;
- collector not enabled;
- estimated/sampled rather than exact.

Render unavailable values as `-` or `?` according to reason. JSON retains the
reason. Never turn missing data into numeric zero.

### 6.5 Policies and dynamic observations are different

Persistent managed selectors should match stable identity/metadata fields such
as executable, app ID, UID, cgroup, systemd unit, container ID, or namespace
inode. Rate metrics are rejected in a persistent control selector by default.

Dynamic metric-based control can flap and is dangerous. It may be introduced as
an explicit later feature with hysteresis, minimum match duration, cooldown, and
clear opt-in; it is not required for MVP.

### 6.6 Managed membership is not control

A target may be managed only for visibility:

```toml
[[managed]]
name = "Browsers"
selector = 'app in ["google-chrome", "chromium", "firefox"]'
```

Control exists only if a `[managed.control]` block or an explicit interactive
action is present.

## 7. User-facing modes

### 7.1 Default command: TUI

```bash
pm
pm tui
pm tui --preset battery
```

If stdout is not a terminal, bare `pm` must fail with a helpful message rather
than emit terminal control sequences. `pm ps` and `pm stat` are the explicit
non-interactive modes.

### 7.2 One-shot table

```bash
pm ps
pm ps --group-by comm --leaf none --sort cpu:desc
pm ps --group-by none --leaf process
pm ps --select 'uid == 1000' --having 'disk-rbps > 20K'
```

The command takes two samples when any requested metric is a rate. The default
warm-up delay equals the process collector interval. `--instant` skips the
second sample and renders rate metrics unavailable.

### 7.3 Streaming, vmstat-like mode

```bash
pm stat 2s
pm stat --interval 500ms --preset io
pm stat 2s --group-by pidns,comm --leaf none \
  --having 'disk-rbps > 20K && disk-wbps < 600K'
```

Requirements:

- emit a timestamp on every row;
- print the header initially and again every configurable number of lines;
- never clear or rewrite prior lines;
- flush after each sample batch;
- handle terminal width but retain stable machine output with `--format csv`,
  `json`, or `ndjson`;
- `Ctrl-C` exits cleanly with code 130;
- `--count N` emits N sample batches and exits;
- `--no-warmup` may emit the first batch with rates unavailable.

### 7.4 Managed view

```bash
pm managed
pm managed --all
pm inspect managed:browsers
```

Inactive logical targets remain visible:

```text
TARGET       P/T      CTL  NICE     STATE     LAST SEEN
Browsers     37/214   N    10←0     ACTIVE    now
pod:foo       0/0     F    -        INACTIVE  06:44:17
```

### 7.5 Machine output

```bash
pm ps --format json
pm stat 1s --format ndjson
pm capabilities --format json
pm metrics list --format json
```

JSON output is a versioned public interface. TUI/table labels are not.

## 8. CLI specification

The examples below are normative behavior; exact flag parsing library is not.

```text
pm [global flags] [command]

Commands:
  tui                     Interactive explorer (default on a TTY)
  ps                      One-shot snapshot/table
  stat [interval]         Repeated append-only samples
  inspect <target>        Detailed entity/group/managed-target inspection
  managed                 List retained managed targets and bindings
  manage <target...>      Add runtime managed membership, optionally control
  set <target...>         Apply/update control while retaining managed state
  restore <target...>     Restore captured original control fields
  unmanage <target...>    Remove membership/policy runtime overlay
  signal <sig> <target>   Send a signal after target resolution and safeguards
  config check [file]     Strictly validate configuration
  config convert <file> --to <toml|yaml|json>
  config dump --effective
  metrics list            List metrics, cost, source, and availability
  capabilities            Explain available/missing collectors and controllers
  daemon run              Run the per-user engine daemon
  service install         Write a user systemd unit (optional feature)
  service uninstall       Remove the generated user unit
  version
```

### 8.1 Global query flags

```text
--config PATH
--no-config
--preset NAME
--interval DURATION
--collector-interval NAME=DURATION   repeatable
--metrics PROFILE
--metric [+|-]NAME                   repeatable
--group-by DIM[,DIM...]
--leaf process|thread|none
--select EXPR
--having EXPR
--sort FIELD[:asc|desc][,...]
--columns FIELD[,FIELD...]
--format table|wide|json|ndjson|csv
--no-color
--strict-capabilities
--state-dir PATH
--socket PATH
--standalone
```

For compatibility, `--where` may be accepted as an alias of `--having`, but
help and documentation should prefer the explicit pair `--select` and
`--having`.

### 8.2 Metric override semantics

```bash
pm --metrics light --metric +disk-rbps --metric -vsz
pm --metrics none --metric cpu --metric rss --metric disk-wbps
```

Overrides are evaluated from left to right after expanding the selected
profile. `--metric cpu` is equivalent to `--metric +cpu`. Unknown metrics are
errors.

### 8.3 Grouping and leaf semantics

```bash
pm ps --group-by namespace-set,app,session --leaf process
pm ps --group-by comm --leaf none
pm ps --group-by none --leaf process
pm ps --group-by pidns --leaf none
```

Rules:

- `group-by` is an ordered list of dimensions, not a hardcoded process tree;
- `none` must be the only group-by value;
- a dimension cannot occur twice;
- `leaf=thread` implies thread enumeration;
- `leaf=process` renders processes below the final group, unless grouping is
  `none`, in which case processes are the top-level rows;
- `leaf=none` renders only group rows;
- if both grouping and leaves are `none`, render one host-total row;
- sorting is applied among siblings, independently at each tree level;
- group order remains the configured dimension order.

### 8.4 Sorting

Accepted forms:

```bash
pm ps --sort cpu:desc,wakeups:desc
pm ps --sort -cpu,+comm
```

Canonical config representation is an ordered array of `{field, direction}`.
Sort must be stable. Final deterministic tie-breakers are row kind, display
name, then internal stable key. Unavailable values sort last in both directions
unless explicitly configured otherwise.

### 8.5 Target syntax

Commands that act on targets accept explicit typed references:

```text
pid:1234
tid:1234/1250
managed:browsers
group:app=google-chrome
group:pidns=4026533012
selector:comm=="chrome"
```

Bare numeric arguments may be accepted as PIDs. Bare strings must not silently
guess between app, comm, executable, and managed target names. The TUI already
has an exact selected row and does not need string parsing.

For shell safety, selectors should normally be single-quoted.

### 8.6 Control commands

```bash
pm manage group:app=google-chrome
pm manage group:app=google-chrome --follow --nice 10
pm manage pid:1234 --snapshot --stop
pm set managed:browsers --nice 15
pm set managed:browsers --freeze
pm restore managed:browsers
pm restore managed:browsers --field nice
pm unmanage managed:browsers
pm signal TERM pid:1234
```

Flags:

```text
--nice N                 Linux nice value -20..19
--stop                   Desired execution control SIGSTOP
--continue               Clear pm-owned SIGSTOP intent
--freeze                 Desired cgroup frozen state
--thaw                   Clear pm-owned cgroup freeze intent
--snapshot               Bind only instances resolved now
--follow                 Re-resolve a logical target continuously
--dry-run
--yes                    Skip interactive confirmation where allowed
--force-system           Override system-process safeguards
--on-drift report|reapply|adopt|ignore
```

Defaults:

- explicit `pid:` and `tid:` targets default to `snapshot`;
- `managed:`, `group:`, and `selector:` targets default to `follow`;
- persistent config rules always follow;
- destructive signals require confirmation on a TTY unless `--yes`;
- non-interactive destructive actions require `--yes`;
- `manage` without control only creates membership.

### 8.7 Exit codes

| Code | Meaning |
|---:|---|
| 0 | Success, including an empty query result. |
| 1 | Generic runtime error. |
| 2 | CLI or configuration validation error. |
| 3 | Requested capability unavailable under this kernel/build. |
| 4 | Permission denied for at least one required operation. |
| 5 | Target not found or no longer exists. |
| 6 | Partial control success across a multi-instance target. |
| 7 | State conflict, stale instance, or external drift blocked the operation. |
| 10 | Daemon communication/protocol error. |
| 130 | Interrupted by SIGINT. |

Machine output must also include per-target errors; exit code alone is not
sufficient for partial operations.

## 9. Configuration

### 9.1 Supported formats

TOML, YAML, and JSON are supported from the first release through decoder
adapters feeding one canonical configuration model.

```text
.toml          → strict TOML decoder ─┐
.yaml / .yml   → strict YAML decoder ─┼→ raw canonical DTO
.json          → strict JSON decoder ─┘
                                      → normalize
                                      → semantic validation
                                      → resolved Config
```

Although YAML 1.2 can parse JSON, JSON must have its own adapter/policy so JSON
users receive JSON-appropriate syntax errors and strict duplicate-key handling.
Formats must have identical semantics.

### 9.2 File discovery

Precedence for selecting a config file:

1. `--no-config`: load none;
2. `--config PATH`;
3. `$PM_CONFIG` if set;
4. exactly one of:
   - `$XDG_CONFIG_HOME/pm/config.toml`;
   - `$XDG_CONFIG_HOME/pm/config.yaml`;
   - `$XDG_CONFIG_HOME/pm/config.yml`;
   - `$XDG_CONFIG_HOME/pm/config.json`;
   - with `~/.config` as the XDG fallback.

If multiple default config files exist, return an error listing them. Never
choose one by extension priority. An absent default config is not an error.

### 9.3 Merge precedence

```text
built-in defaults
  < config defaults
  < selected preset and its ancestors
  < CLI flags, in CLI order
```

Managed rules are a separate policy list and are not replaced by selecting a
view preset.

Scalar values replace. Maps merge by key. For ordinary arrays, a child value
replaces its parent. Metrics use explicit `profile`, `enable`, and `disable`
semantics. Preset inheritance must reject cycles and missing parents.

### 9.4 Strictness

All formats must reject:

- unknown fields;
- duplicate keys;
- type mismatches;
- invalid enums and metric/dimension names;
- invalid or negative durations;
- bad selector syntax or invalid field use;
- duplicate preset/rule names;
- preset inheritance cycles;
- impossible combinations such as `group_by=["none","app"]`;
- control rules that reference unsupported dynamic fields, unless an explicit
  future unsafe option enables them.

Warnings are appropriate for available-but-permission-denied runtime features,
not for malformed intent.

### 9.5 Canonical TOML example

```toml
version = 1
interval = "1s"
group_by = ["app", "session"]
leaf = "process"
columns = ["target", "pt", "cpu", "rss", "disk-rbps", "disk-wbps", "ctl", "pstate", "pnice"]

[intervals]
process = "1s"
fd = "5s"
socket = "5s"
ebpf = "1s"
perf = "10s"

[metrics]
profile = "light"
enable = ["disk-rbps", "disk-wbps"]
disable = ["vsz"]

[[sort]]
field = "cpu"
direction = "desc"

[state]
runtime_dir = ""
history = false

[daemon]
use = "auto"
socket = ""

[presets.battery]
interval = "2s"
group_by = ["app"]
leaf = "process"
columns = ["target", "pt", "cpu", "wakeups", "timer-wakeups", "rss", "ctl"]

[presets.battery.metrics]
profile = "light"
enable = ["wakeups", "timer-wakeups"]

[[presets.battery.sort]]
field = "wakeups"
direction = "desc"

[presets.io]
interval = "1s"
group_by = ["comm"]
leaf = "none"
having = "disk-rbps > 0 || disk-wbps > 0"
columns = ["target", "procs", "disk-rbps", "disk-wbps", "read-syscalls", "write-syscalls"]

[presets.io.metrics]
profile = "io"

[presets.io-heavy]
extends = "io"
having = "disk-rbps > 20M || disk-wbps > 20M"

[[managed]]
name = "Browsers"
selector = 'app in ["google-chrome", "chromium", "firefox"]'

[managed.view]
group_by = ["app", "session"]

[[managed]]
name = "Background IDE"
selector = 'app == "idea" || (comm == "java" && cmdline contains "idea")'
on_drift = "report"

[managed.control]
nice = 10
```

Equivalent YAML and JSON examples should be generated from the canonical model
in documentation tests rather than maintained manually.

### 9.6 Built-in presets

The binary ships read-only presets:

| Preset | Intent |
|---|---|
| `default` | Balanced process view using light metrics. |
| `battery` | App grouping and wakeup/power-relevant columns; advanced values appear only if available. |
| `io` | Block I/O bytes, syscall counts, faults, and I/O delay. |
| `churn` | Process/thread births and deaths over short windows. |
| `namespaces` | Namespace-set and cgroup exploration. |
| `managed` | Control state, original/desired/observed values, and drift. |
| `all` | All compiled and available metrics; explicitly expensive. |

Custom presets may override built-ins by name only if
`allow_builtin_preset_override = true`; otherwise duplicate names are errors.

### 9.7 Config utilities

```bash
pm config check
pm config check ./config.yaml
pm config convert config.yaml --to toml
pm --preset battery --interval 500ms config dump --effective
```

`dump --effective` must show the fully resolved canonical configuration after
defaults, config, preset inheritance, and CLI overrides. It should optionally
annotate each value's source with `--explain`.

## 10. Selector and filter expression language

### 10.1 Two evaluation stages

- `select`: evaluated for each normalized entity before grouping. It determines
  membership in the query.
- `having`: evaluated on rendered groups/leaves after aggregation. It determines
  which rows remain visible.

This distinction prevents ambiguous queries such as `cpu > 5` when CPU could
mean a process value or an aggregate group value.

Persistent `managed[].selector` uses the entity-selector evaluator.

### 10.2 Grammar

An implementation may use a parser generator or hand-written Pratt parser. The
public grammar is:

```ebnf
expr        = or_expr ;
or_expr     = and_expr { "||" and_expr } ;
and_expr    = unary_expr { "&&" unary_expr } ;
unary_expr  = [ "!" ] primary ;
primary     = "(" expr ")"
            | comparison
            | function_call
            | boolean_field ;
comparison  = value ( "==" | "!=" | "<" | "<=" | ">" | ">=" |
                      "~=" | "!~" | "in" | "not in" | "contains" ) value ;
function_call = ident "(" [ value { "," value } ] ")" ;
value       = field | string | number | size | duration | boolean | list ;
list        = "[" [ value { "," value } ] "]" ;
field       = ident { "." ident } ;
```

### 10.3 Literals

- Strings use double quotes with JSON-style escapes.
- Integers and decimals are supported.
- Byte sizes support decimal and binary suffixes: `20K`, `20KB`, `20KiB`,
  `1G`, `1GiB`.
- Rates are represented by the metric field, not a `/s` suffix in the literal.
- Durations use Go-like units: `500ms`, `2s`, `1m`, `1h`.
- Regex operator `~=` uses RE2 syntax to avoid catastrophic backtracking.
- Enums may be bare identifiers only where the field type makes them
  unambiguous; quoted strings remain universally valid.

### 10.4 Functions

Initial functions:

```text
exists(field)        value is available in this row
missing(field)       value is unavailable
changed(field)       pm has changed this controlled field
drifted(field)       desired and observed differ
age()                process/group age
last_seen()          managed target's last-seen age
```

Potential later functions such as rolling averages must not be accepted until
their window semantics are implemented.

### 10.5 Type checking

Expressions are parsed and type-checked before sampling starts. The compiler
must know whether fields are valid for entity or aggregate evaluation. Examples:

```text
comm ~= "^(chrome|chromium)$"
uid == 1000 && pidns != host_pidns
(cpu > 5 || wakeups > 100) && disk-wbps < 600K
managed == true && drifted(nice)
```

Invalid comparisons, unknown fields, aggregate-only fields in `select`, or
entity-only fields in a group-only `having` are configuration/CLI errors.

### 10.6 Missing values

Ordinary comparison with a missing value evaluates to false, including `!=`.
Users must call `missing(field)` or `exists(field)` when availability matters.
Boolean short-circuiting is required.

## 11. Domain model

The names below are illustrative Go types but their separation is normative.

```go
type ProcessInstanceID struct {
    BootID    string
    PIDNSIno  uint64
    PID       int
    StartTime uint64 // clock ticks since boot from /proc/PID/stat
}

type ThreadInstanceID struct {
    Process ProcessInstanceID
    TID     int
    StartTime uint64
}

type Availability string

const (
    Available        Availability = "available"
    WarmingUp        Availability = "warming_up"
    Disabled         Availability = "disabled"
    Unsupported      Availability = "unsupported"
    PermissionDenied Availability = "permission_denied"
    Vanished         Availability = "vanished"
    ReadError        Availability = "read_error"
)

type Quality string

const (
    Exact     Quality = "exact"
    Sampled   Quality = "sampled"
    Estimated Quality = "estimated"
    Derived   Quality = "derived"
)

type Value[T any] struct {
    Value        T
    Availability Availability
    Quality      Quality
    Source       string
}
```

### 11.1 Normalized process entity

At minimum:

```go
type Process struct {
    ID ProcessInstanceID

    PID, TGID, PPID, PGID, SID int
    UID, EUID, GID, EGID       uint32

    Comm       string
    Cmdline    []string
    Exe        string
    Cwd        string
    CgroupPath string
    SystemdUnit string
    AppID      string
    ContainerID string
    PodUID      string

    Namespaces NamespaceSet
    State      ProcessState
    Flags      ProcessFlags

    Metrics map[MetricID]MetricValue
}
```

Sensitive or permission-restricted fields keep availability metadata. JSON must
distinguish an empty cmdline from an unreadable cmdline.

### 11.2 Namespace identity

Each namespace is represented by type and inode:

```go
type NamespaceID struct {
    Type  NamespaceType // pid, mnt, net, user, uts, ipc, cgroup, time
    Inode uint64
}
```

`namespace-set` is a tuple of the enabled namespace IDs and therefore groups
processes sharing the whole container-like isolation boundary. `pidns`, `netns`,
etc. are independent grouping dimensions.

Friendly labels are resolver metadata, never native identity. Resolvers may use:

- container runtime metadata;
- cgroup paths;
- Kubernetes pod UID/name metadata;
- systemd unit names;
- a user alias map in config.

When no label exists, display `pidns:4026533012`, not a fabricated name.

### 11.3 Process state

Do not store `SslT` as a single opaque state. Model:

```go
type ProcessState struct {
    Code rune // R, S, D, T, t, Z, X, I, ...
}

type ProcessFlags struct {
    SessionLeader bool
    Multithreaded bool
    Foreground    bool
    HighPriority  bool
    LowPriority   bool
    LockedMemory  bool
}
```

The renderer composes a familiar `ps`-style `PSTATE`, while filters can address
the structured fields.

### 11.4 Logical target identity

```go
type ManagedTarget struct {
    ID          string
    Name        string
    Origin      TargetOrigin // config_policy, runtime_manual
    BindingMode BindingMode  // follow, snapshot
    Selector    CompiledSelector
    SnapshotIDs []ProcessInstanceID
    Control     DesiredControl
    LastSeen    time.Time
    Counters    LifetimeCounters
}
```

Logical identity preference for automatically named targets:

1. configured managed rule ID/name;
2. Kubernetes pod UID or container ID;
3. systemd unit and user scope;
4. desktop app ID plus executable;
5. normalized executable path;
6. namespace instance inode;
7. explicit snapshot instance ID.

Namespace inode reuse after an instance disappears must not automatically bind
historical state across boots. `boot_id` scopes ephemeral namespace identity.

## 12. Grouping and aggregation

### 12.1 Initial dimensions

```text
host
namespace-set
pidns
mntns
netns
userns
cgroupns
cgroup
systemd-unit
container
pod
app
exe
comm
uid
user
session
process-group
process
thread
```

Dimensions have a stable machine ID, display label, key type, and availability.
Optional metadata resolvers may add labels but not change keys.

### 12.2 Counts

Every group maintains:

- `children`: immediate child row count;
- `procs`: recursively distinct process instances;
- `threads`: recursively distinct thread instances;
- `leaves`: rendered leaf count under current leaf mode.

Default compact column `P/T` displays `procs/threads`. Expanded columns may show
all four counts.

### 12.3 Metric aggregation registry

Aggregation is metric metadata, not renderer logic:

| Metric class | Default group aggregate |
|---|---|
| Additive counters/rates | Sum |
| CPU time / CPU rate | Sum |
| RSS, VSZ for process entities | Sum once per distinct process |
| Process RSS shown under thread leaves | Inherited/display-only; never summed per thread |
| Nice | Mixed-value set or min/max; no numeric sum |
| State | Counts by state plus derived summary |
| Age | Oldest process age by default |
| Availability | Available count plus missing-reason counts |
| Boolean control flags | Any/all plus mixed status |

Every metric definition declares valid entity kinds, unit, monotonicity,
counter width, delta behavior, and aggregate function.

### 12.4 Avoiding double counting

Thread enumeration creates a common trap: `/proc/PID/task/TID` repeats
process-scoped memory and I/O information. The normalized model must tag metric
scope as host, process, or thread. Aggregators deduplicate process-scoped values
by `ProcessInstanceID`.

### 12.5 Churn attribution

Polling-based churn compares identity sets between successful scans:

- `pids-born`, `pids-died`;
- `tids-born`, `tids-died`;
- per-second rates;
- rolling windows of 1s, 10s, and 1m where enabled.

Events that begin and end entirely between scans are invisible and the metric
quality is `sampled`. An event-backed collector may later expose exact churn.
Deaths are attributed using the last known entity metadata. Births use the first
successful observation.

## 13. Metrics registry

Metric IDs are stable kebab-case public identifiers. Column aliases may be
shorter but JSON and config use canonical IDs.

### 13.1 Cost level 0: light `/proc` metrics

| Metric ID | Unit | Source / note |
|---|---:|---|
| `cpu` | percent of one CPU | Delta of process user+system ticks; may exceed 100 on multi-threaded process. |
| `cpu-normalized` | percent of host capacity | `cpu / online_cpu_count`. |
| `cpu-user` | percent of one CPU | Delta of `utime`. |
| `cpu-system` | percent of one CPU | Delta of `stime`. |
| `rss` | bytes | Resident pages from `/proc/PID/statm` or status. |
| `vsz` | bytes | Virtual memory size. |
| `threads` | count | `/proc/PID/status`. |
| `minor-faults` | per second | Delta of `minflt`. |
| `major-faults` | per second | Delta of `majflt`. |
| `ctxsw-voluntary` | per second | Delta from status. |
| `ctxsw-involuntary` | per second | Delta from status. |
| `disk-read-bytes` | bytes/sec | Delta of `/proc/PID/io read_bytes`; actual storage-accounted bytes when supported. |
| `disk-write-bytes` | bytes/sec | Delta of `write_bytes`. |
| `io-rchar` | bytes/sec | Delta of `rchar`; includes cached/terminal I/O. |
| `io-wchar` | bytes/sec | Delta of `wchar`. |
| `read-syscalls` | calls/sec | Delta of `syscr`; not physical disk operations. |
| `write-syscalls` | calls/sec | Delta of `syscw`; not physical disk operations. |
| `cancelled-write-bytes` | bytes/sec | Delta where exposed. |
| `blkio-delay` | time/sec or percent | Delta of delay accounting ticks if enabled. |
| `pids-born`, `pids-died` | count/rate | Snapshot set difference, sampled quality. |
| `tids-born`, `tids-died` | count/rate | Requires thread enumeration. |
| `pstate` | enum/flags | Current kernel state and derived flags. |
| `pnice` | integer | Actual observed nice. |
| `age` | duration | Derived from starttime and boot clock. |

Public aliases `disk-rbps` and `disk-wbps` may map to
`disk-read-bytes`/`disk-write-bytes`, but one canonical ID must be chosen before
v1. This RFC recommends retaining the shorter `disk-rbps` and `disk-wbps` as
canonical because they appear frequently in interactive use.

### 13.2 Cost level 1: periodic FD/socket inspection

Default interval: 5 seconds.

```text
fd-total
fd-files
fd-sockets
fd-pipes
fd-anon
fd-eventfd
fd-epoll
fd-timerfd
fd-signalfd
fd-inotify
socket-tcp
socket-udp
socket-unix
socket-netlink
```

Implementation scans `/proc/PID/fd` symlinks and correlates socket inodes with
procfs/netlink socket tables. Races and permission failures are expected.
Counts should carry sampled quality.

### 13.3 Cost level 2: event tracing, usually eBPF

```text
wakeups
timer-wakeups
futex-wakes
poll-wakes
scheduler-migrations
runqueue-delay
open-rate
close-rate
read-rate
write-rate
fsync-rate
fdatasync-rate
connect-rate
accept-rate
net-rx-bps
net-tx-bps
net-rx-pps
net-tx-pps
```

Each metric needs a written attribution definition. For example, a scheduler
wakeup may be attributed to the awakened task, the waking task, or both under
different IDs. Do not ship an ambiguous `wakeups` counter.

Per-process network bytes are not obtainable reliably from ordinary
`/proc/PID` scanning. They remain unavailable unless a suitable eBPF or other
accounting backend is active.

### 13.4 Cost level 3: perf/hardware counters

```text
cycles
instructions
ipc
cache-misses
context-switches-perf
```

These are opt-in, may require `perf_event_paranoid` changes or capabilities,
and should use a slower default interval. Multiplexing/scaling metadata must be
reported.

### 13.5 Metric profiles

Profiles are convenience sets, not special code paths:

```text
none
light
process
io
network
power
perf
all
```

`all` means all compiled and currently supported metrics, not that unavailable
metrics become zero.

### 13.6 Sampling and rates

- Use `CLOCK_BOOTTIME` or a monotonic clock for elapsed time.
- Rate denominator is actual elapsed monotonic duration, not configured
  interval.
- First counter sample is `warming_up`.
- Counter decrease means process replacement, reset, or backend reset; discard
  the delta unless wrap behavior is explicitly known.
- A collector that misses a deadline must not pretend the nominal interval
  elapsed.
- Keep timestamp and collection duration for every sample batch.
- Coalesce duplicate procfs reads needed by multiple metrics.

## 14. Collector architecture

```go
type Collector interface {
    ID() string
    Describe() CollectorDescriptor
    Probe(context.Context) CapabilityStatus
    Collect(context.Context, CollectRequest) (CollectorSample, error)
}
```

The collector scheduler:

- resolves requested metrics to the minimum required collector set;
- schedules independent intervals;
- shares the current process identity inventory;
- publishes immutable sample generations;
- records collection duration, skipped deadlines, errors, and permission
  failures;
- avoids concurrent duplicate reads of the same proc file;
- applies bounded concurrency so a large PID count cannot create an I/O storm.

### 14.1 procfs reader

Procfs access must be abstracted behind an interface rooted at a configurable
path. Tests can then mount fixtures or use a fake proc tree. Parsing
`/proc/PID/stat` must correctly handle command names containing spaces and
parentheses: locate the final `)` before splitting subsequent fields; do not use
a naive whitespace split over the entire line.

A process may disappear between any two reads. `ENOENT`/`ESRCH` is a normal
vanish outcome, not a noisy log error.

### 14.2 Metadata resolvers

Resolvers decorate normalized entities and dimensions:

```go
type Resolver interface {
    ID() string
    Resolve(context.Context, *Process) error
}
```

Initial resolvers:

- passwd UID/GID names with caching;
- cgroup v2 path;
- systemd unit heuristic from cgroup path;
- desktop application heuristic from executable/cmdline and `.desktop` files;
- namespace user aliases from config.

Container runtime and Kubernetes resolution are optional later plugins. A
failed resolver must not invalidate the base process.

### 14.3 Capability probing

Probe once at startup and refresh after relevant errors. Report:

- procfs presence and hidepid restrictions;
- cgroup version and writable controllers;
- pidfd availability;
- task delay accounting;
- BPF syscall/features and effective privileges;
- perf event access;
- namespace types present;
- daemon/socket availability.

`pm capabilities` must include a human explanation and remediation hints without
requiring the user to run as root by default.

## 15. Control model

### 15.1 Initial controllers

| Control | Backend | Initial phase |
|---|---|---|
| Nice | `getpriority`/`setpriority` | MVP |
| Stop/continue | pidfd signal or `kill(SIGSTOP/SIGCONT)` | MVP |
| Freeze/thaw | cgroup v2 `cgroup.freeze` | Phase 2 |
| CPU weight/max | cgroup v2 | Phase 2 |
| I/O priority | `ioprio_get`/`ioprio_set` | Phase 2 |
| CPU affinity | `sched_getaffinity`/`sched_setaffinity` | Phase 2 |
| Arbitrary signal | pidfd signal or `kill` | MVP, safeguarded |

### 15.2 State tuple per field and instance

```go
type ControlledField[T comparable] struct {
    Original   T
    Desired    T
    Observed   T
    Status     ControlStatus // applied, drifted, unavailable, vanished, restored
    Backend    string
    CapturedAt time.Time
    ChangedAt  time.Time
    ObservedAt time.Time
    Error      string
}
```

Capture `Original` exactly once per target-binding instance and field, before
the first successful `pm` mutation. Updating desired state must not overwrite
original.

### 15.3 Nice semantics

The default table columns are:

- `NICE`: desired arrow original, e.g. `10←0`; `-` if unmanaged;
- `PNICE`: actual observed process nice;
- `CTL`: contains `N` if nice is controlled and `N!`/drift marker if observed
  differs from desired.

For a mixed group, `NICE` renders `mixed`; inspection lists values and counts.

Increasing numeric nice (reducing priority) is usually allowed for owned
processes. Restoring to a smaller number may require `CAP_SYS_NICE`; therefore a
change can succeed while later restore is permission denied. `pm` must warn
before applying a change when it predicts restore may require privileges not
currently available.

### 15.4 STOP/CONT semantics

`CONT` is an action, not a process state. `pm` tracks its own stop intent as
control state while `PSTATE` remains the observed kernel state.

Important limitation: Linux does not expose a clean ownership count for
SIGSTOP reasons. If a task was already stopped before `pm`, `pm` must record that
and must not blindly send SIGCONT during restore. If another actor stops it
after `pm`, ownership is ambiguous. Default drift policy is report and require
confirmation, not force-continue.

### 15.5 Cgroup freeze semantics

Arbitrary process groups do not necessarily correspond to an existing writable
cgroup. A cgroup controller must declare whether it:

- controls an existing cgroup in place; or
- creates a delegated `pm` cgroup and migrates processes.

Migration changes cgroup membership and can conflict with systemd/container
managers. The first cgroup-freeze implementation should operate only on an
existing delegated/writable cgroup. Automatic migration is a separate explicit
feature and must store original cgroup membership.

### 15.6 Applying to multi-instance targets

Group control is best-effort over a changing set:

1. resolve and sort bindings;
2. exclude protected instances;
3. revalidate PID/starttime or acquire pidfd;
4. capture original values;
5. apply per instance;
6. immediately read observed values;
7. atomically persist results;
8. return success, failure, stale, and skipped counts.

There is no false promise of transactionality. `--dry-run` shows the exact
resolved instances and required capabilities.

### 15.7 Reconciliation and drift

Policies and follow-mode targets are reconciled whenever a fresh entity
generation is available. New matching instances capture their own original
values before policy application.

Drift behavior:

| Mode | Behavior |
|---|---|
| `report` | Mark drift; do not overwrite external change. Default. |
| `reapply` | Restore desired value after guard checks. |
| `adopt` | Treat current observed value as new desired; preserve original. |
| `ignore` | Keep desired metadata but suppress drift action/alert. |

`reapply` must be rate-limited and produce an event so two controllers cannot
silently fight at high frequency.

### 15.8 Restore vs unmanage

- `restore`: compare-and-set where practical, returning controlled fields to
  captured original values. Target remains managed unless `--and-unmanage`.
- `unmanage`: remove membership/reconciliation intent. Current kernel state is
  left untouched unless `--restore` is explicitly supplied.

If observed state drifted, default restore refuses to overwrite it and returns
exit code 7. `--force` may restore original after confirmation.

### 15.9 Safeguards

By default refuse control of:

- PID 1;
- kernel threads;
- the `pm` daemon, current client, and their ancestor chain;
- the user's session manager and critical systemd manager processes;
- a target resolving to more than a configurable safety limit, default 256,
  without confirmation;
- processes outside the caller's permission boundary;
- ambiguous namespace/cgroup target identities from another boot.

`--force-system` is explicit, noisy, and still cannot bypass kernel permission
checks. Never implement a setuid path.

## 16. Runtime, persistent policy, and history state

### 16.1 Three state classes

| Class | Default location | Contents | Lifetime |
|---|---|---|---|
| Config / intent | `$XDG_CONFIG_HOME/pm/config.{toml,yaml,yml,json}` | Defaults, presets, managed selectors, policies | Persistent |
| Runtime state | `$XDG_RUNTIME_DIR/pm/` | Live bindings, manual targets, captured originals, desired/observed values, actions | Login session / reboot |
| Optional history | `$XDG_STATE_HOME/pm/` | Audit events and inactive target counters | Persistent, opt-in |

Fallback runtime path is `/run/user/<uid>/pm` only when ownership and mode are
correct. If `XDG_RUNTIME_DIR` is absent and no safe `/run/user/<uid>` exists,
standalone mode may create a private temporary directory and warn that restore
metadata is not durable across invocations. It must never default to a shared,
predictable `/tmp/pm` path.

### 16.2 Runtime files

Recommended layout:

```text
$XDG_RUNTIME_DIR/pm/
  state.json
  state.lock
  pm.sock
  events.ndjson          optional bounded debug log
```

Directory mode is `0700`; files are `0600`; Unix socket is user-only. Refuse a
directory/file owned by another UID or unsafe symlink. Use `openat2`/safe open
patterns where practical.

### 16.3 Runtime state format

JSON is acceptable for v1. It is internal but versioned:

```json
{
  "schema_version": 1,
  "boot_id": "...",
  "owner_uid": 1000,
  "generation": 42,
  "updated_at": "2026-09-12T20:44:17.123Z",
  "targets": [
    {
      "id": "runtime:app:google-chrome",
      "name": "Chrome",
      "origin": "runtime_manual",
      "binding_mode": "follow",
      "selector": "app == \"google-chrome\"",
      "last_seen": "2026-09-12T20:44:17.000Z",
      "control": {"nice": 10},
      "bindings": [
        {
          "boot_id": "...",
          "pidns_inode": 4026531836,
          "pid": 1234,
          "starttime_ticks": 271583500,
          "nice": {
            "original": 0,
            "desired": 10,
            "observed": 10,
            "status": "applied",
            "backend": "setpriority"
          }
        }
      ]
    }
  ]
}
```

Write via temp file, `fsync`, atomic rename, and directory `fsync` where
available. Preserve a `.bak` previous generation if cheap. A corrupt file must
be quarantined, not silently overwritten before the user can inspect it.

### 16.4 Locking and single writer

The daemon is the preferred single runtime-state writer. In standalone mode,
commands take an exclusive advisory lock before mutations. Read-only commands
may take a shared lock or use an immutable snapshot. If the daemon socket is
active, clients must use it rather than mutate the file directly unless
`--standalone` is explicitly requested and safe.

### 16.5 Boot handling

On boot ID mismatch:

- discard concrete PID/TID bindings and captured per-instance originals;
- retain config-defined logical policies by reloading config;
- runtime manual targets naturally disappear with runtime storage;
- persistent history remains history and must not rebind by numeric PID or
  namespace inode alone.

### 16.6 Inactive managed targets

When the final binding disappears:

- remove vanished PID/TID instances immediately from the live entity tree;
- retain the logical managed target;
- set state to `INACTIVE`;
- retain `last_seen`, lifetime, total births/deaths, and last control intent;
- rebind if a follow-mode selector matches a future process;
- never rebind a snapshot target.

## 17. Daemon and standalone execution

### 17.1 Why a daemon exists

Persistent config can express `chrome always has nice=10`, but automatic
enforcement only happens while an engine is running. An optional per-user daemon
provides:

- continuous policy matching and reconciliation;
- shared collection for multiple UI/CLI clients;
- authoritative runtime-state ownership;
- event-driven notifications and lower duplicate overhead.

### 17.2 Modes

```text
daemon=auto     connect if socket exists; otherwise run embedded
daemon=require  fail unless daemon is available
daemon=never    always run embedded/standalone
```

The TUI may offer to show the user-systemd enable command but must not silently
install or enable persistence.

### 17.3 IPC

Use a versioned protocol over an `AF_UNIX` socket. JSON messages are adequate
for MVP; length-prefix frames or NDJSON with strict size limits are acceptable.
The protocol needs:

- hello/version/capabilities;
- subscribe snapshot stream;
- run query specification;
- list/inspect managed state;
- dry-run and execute control plan;
- restore/unmanage;
- reload config;
- health and collector statistics.

Authenticate by socket ownership and peer credentials (`SO_PEERCRED`). Reject a
different UID by default. Bound request and response sizes.

### 17.4 Daemon reload

SIGHUP or `pm daemon reload` performs parse/normalize/validate before replacing
the active config. A bad new config leaves the prior config active and reports
the error. Removed policy rules become unmanaged but are not implicitly
restored; an optional future rule may request restore-on-removal.

## 18. Renderers and columns

### 18.1 Column registry

Columns are registered descriptors containing:

- stable ID and aliases;
- header and long description;
- minimum/preferred width;
- alignment;
- unit and formatter;
- compatible row/entity kinds;
- required metrics/capabilities;
- default aggregate display.

Initial columns:

```text
target, kind, pid, tid, ppid, uid, user, app, comm, exe,
namespace-set, pidns, netns, cgroup, systemd-unit,
children, leaves, procs, threads, pt,
cpu, cpu-user, cpu-system, rss, vsz,
minor-faults, major-faults, ctxsw-voluntary, ctxsw-involuntary,
disk-rbps, disk-wbps, read-syscalls, write-syscalls, blkio-delay,
wakeups, timer-wakeups, net-rx-bps, net-tx-bps,
ctl, nice, onice, pnice, pstate, managed, drift, active, last-seen
```

### 18.2 Control columns

`CTL` is a compact summary:

```text
N  nice controlled
S  SIGSTOP intent
F  cgroup frozen
C  CPU controller changed
I  I/O priority/weight changed
A  CPU affinity changed
!  at least one controlled field drifted or is ambiguous
?  control state partly unavailable
```

Examples: `N`, `SN`, `N!`, `NCI`. Detailed inspection always uses named fields;
the letters are presentation only.

### 18.3 Table example

```text
GROUP/PROCESS              P/T      CPU  WAKE/s    RSS    DISK R/W    NET R/W  CTL  PSTATE PNICE
HOST                    214/941     8.1     731    14G    1M/3M      90K/41K   -
  chrome                 37/214     2.3     402   3.8G   10K/120K    31K/18K   N
    session:28191        18/103     1.7     321   2.1G                            N
      chrome              12/68     1.1     219   1.3G                            N    Ssl       10
  idea                     9/87     4.7     291   5.2G  320K/2M                   N!
```

Headers must make units explicit in wide or help output. Table formatting must
not be parsed as a stable API.

### 18.4 JSON schema shape

Each response includes:

```json
{
  "schema_version": 1,
  "generated_at": "...",
  "monotonic_elapsed_ns": 1000123456,
  "query": {},
  "capabilities": {},
  "rows": []
}
```

Metric JSON values include `value`, `unit`, `availability`, `quality`, and
`source`, unless a compact schema is explicitly requested. NDJSON emits one
metadata record followed by timestamped sample records.

## 19. TUI specification

### 19.1 Layout

The default TUI has:

1. status bar: host, sample age, interval, metric profile, collector warnings;
2. main tree/table;
3. optional managed panel;
4. details/help overlay.

Tree expansion state is keyed by stable group keys and survives refreshes.
Vanished process/thread rows disappear immediately; inactive managed targets
remain only in the managed panel.

### 19.2 Required interactions

```text
q / Ctrl-C      quit
?               help
g               edit/reorder grouping dimensions
l               choose leaf mode
M               metrics/profile picker
c               column picker
f or /          entity/row filter editor
s               sort picker
r               force refresh
[ / ]           decrease/increase base interval
m               manage/unmanage selected row
n               set nice
Space           stop/continue selected target
F               freeze/thaw selected cgroup target
R               restore selected target
i / Enter       inspect/details
Tab             switch main/managed panel
```

Mouse support is optional for MVP. When implemented, clicking a column cycles
ascending/descending; Shift-click adds or changes a secondary sort key.

### 19.3 Confirmation and previews

Before a group control action, show:

- logical target and binding mode;
- exact matching process count;
- protected/skipped count;
- control values to capture/change;
- predicted privilege/restore limitations.

The user may inspect the dry-run list before confirming.

### 19.4 Responsiveness

Sampling and rendering run independently. Slow collectors must not block input.
The UI displays the last complete generation and marks stale columns. Terminal
resize and config reload must not restart collectors unnecessarily.

## 20. Permissions and security

### 20.1 Principle

Run as the invoking user and use the permissions the kernel already grants. Do
not recommend root as the default solution and do not ship setuid code.

### 20.2 Expected restrictions

- procfs `hidepid` may hide other users' processes;
- `/proc/PID/io`, `exe`, `fd`, and `cmdline` can be restricted;
- signaling and priority changes require ownership/capabilities;
- lowering numeric nice values may require `CAP_SYS_NICE`;
- cgroup writes require delegation or privileges;
- eBPF may require `CAP_BPF`, `CAP_PERFMON`, `CAP_SYS_ADMIN`, or appropriate
  kernel policy;
- perf counters may be blocked by `perf_event_paranoid`.

Each capability reports `available`, `unsupported`, or `permission_denied` with
a reason. `--strict-capabilities` turns unavailable explicitly requested metrics
into exit code 3/4; default operation continues with unavailable values.

### 20.3 Sensitive output

Command lines, environment, CWD, and file paths may contain secrets. `pm` does
not collect process environments by default. Machine output should provide
`--redact cmdline|paths|all`. The daemon socket and state directory are
user-private.

### 20.4 Config trust

Config is code-like user intent because managed policies can stop or reprioritize
processes. Refuse config files writable by another user in daemon/policy mode,
unless an explicit unsafe flag is given. Merely viewing processes may tolerate
looser config with a warning.

## 21. Error handling and race behavior

### 21.1 Procfs races

Expected outcomes such as process exit during sampling are aggregated into
collector statistics, not logged per occurrence at warning level. Unexpected
parse errors include PID/path context and a bounded raw excerpt in debug logs.

### 21.2 PID reuse

Before any mutation:

1. open/acquire pidfd if supported;
2. read and compare starttime to the planned identity;
3. act through pidfd where the syscall allows;
4. otherwise act by PID and immediately re-read identity/state;
5. if validation fails, mark stale and do not retry on the new process.

### 21.3 Partial samples

Snapshot generations are immutable and may contain per-field unavailable values.
A collector-wide failure does not discard successful base identity collection.

### 21.4 Partial control

Return a structured result per binding:

```text
applied
unchanged
skipped-protected
stale
permission-denied
failed
vanished
```

Persist successes and failures so restore and diagnostics reflect reality.

## 22. Performance budgets

Budgets are measured on a representative Linux laptop with 300 processes and
1,500 threads, after warm-up:

- `light`, 1s process interval, process leaves:
  - average CPU below 0.5% of one core over 60 seconds;
  - resident memory below 40 MiB;
  - p95 complete base scan below 100 ms;
- TUI render/input latency below 100 ms when collectors are slow;
- FD collector at 5s must use bounded concurrency and never keep more than 32
  proc directory scans in flight;
- disabled collectors perform no periodic work;
- daemon with two clients performs one shared collection, not two;
- self-metrics are visible in `pm capabilities --verbose` or debug inspection.

These are initial engineering targets, not a reason to corrupt or omit data.
Benchmark results should accompany significant collector changes.

## 23. Logging and diagnostics

Default CLI/TUI output stays quiet. Diagnostic logging supports:

```text
--log-level error|warn|info|debug|trace
--log-format text|json
--log-file PATH
```

Never mix logs into JSON/NDJSON stdout; logs go to stderr or the configured
file. The daemon exposes:

- scan duration and entity count;
- per-collector duration/error/skip count;
- query/render duration;
- state generation and last successful write;
- reconcile actions, failures, and drift;
- connected clients and protocol versions.

An optional `pm doctor` command may later package non-sensitive diagnostic
metadata. It must redact cmdlines and paths by default.

## 24. Internal package layout

Recommended repository structure:

```text
cmd/pm/                   entry point
internal/app/             command orchestration
internal/config/          adapters, canonical DTO, merge, normalize, validate
internal/expr/            lexer, parser, type checker, evaluator
internal/model/           entities, identities, metric/control values
internal/procfs/          safe procfs reading and parsing
internal/collect/         registry, scheduler, collectors
internal/resolve/         app/systemd/cgroup/namespace label resolvers
internal/query/           select, grouping, aggregate, having, sort, projection
internal/metrics/         metric descriptors and profiles
internal/control/         planner, safeguards, controllers, restore/reconcile
internal/state/           runtime persistence, locks, history interface
internal/daemon/          engine lifecycle and IPC server
internal/client/          IPC and embedded-engine facade
internal/render/table/    one-shot and streaming tables
internal/render/json/     versioned machine formats
internal/tui/             interactive UI
internal/testutil/        fake clock, proc fixtures, fake controllers
docs/                     user and architecture documentation
```

Keep Linux syscalls behind small interfaces. Avoid a package named `util` as a
dumping ground.

## 25. Recommended Go interfaces

```go
type MetricDescriptor struct {
    ID           MetricID
    Unit         Unit
    Scope        MetricScope
    Kind         ValueKind
    Cost         CostLevel
    Collector    string
    Monotonic    bool
    Aggregation  AggregationKind
    Description  string
}

type QuerySpec struct {
    Metrics   MetricSelection
    Select    *expr.Program
    GroupBy   []GroupDimension
    Leaf      LeafMode
    Having    *expr.Program
    Sort      []SortKey
    Columns   []ColumnID
}

type Engine interface {
    Snapshot(context.Context, QuerySpec) (*QueryResult, error)
    Subscribe(context.Context, QuerySpec) (<-chan QueryResult, error)
    Capabilities(context.Context) CapabilityReport
}

type Controller interface {
    Plan(context.Context, TargetSpec, DesiredControl) (ControlPlan, error)
    Apply(context.Context, ControlPlan) ControlResult
    Restore(context.Context, RestorePlan) ControlResult
    Observe(context.Context, []ProcessInstanceID) []ObservedControl
}
```

The TUI and CLI should depend on client-facing interfaces, not concrete daemon
or collector packages.

## 26. Testing strategy

### 26.1 Unit tests

- `/proc/PID/stat` commands containing spaces, `)`, and unusual bytes;
- PID/starttime identity and boot changes;
- counter delta, reset, elapsed-time, and warming-up behavior;
- config equivalence across TOML/YAML/JSON;
- strict duplicate and unknown key rejection in all formats;
- preset inheritance, merge order, and cycle rejection;
- selector grammar, precedence, typing, missing values, units, regex;
- grouping pipelines and deterministic keys;
- process-scoped metric deduplication under thread leaves;
- aggregation and mixed controlled state;
- sort stability and unavailable ordering;
- state atomic write/recovery and unsafe ownership checks;
- original/desired/observed transitions and drift policies;
- restore vs unmanage semantics.

### 26.2 Property/fuzz tests

- procfs stat/status/io parsers never panic;
- expression parser/evaluator never panic and respects short-circuiting;
- config decoder round-trips canonical values;
- grouping does not lose or duplicate distinct entity identities;
- state decoding rejects malformed or oversized input safely.

### 26.3 Integration tests

Run in a disposable user namespace/container where available:

- spawn sleeping, CPU-bound, I/O, multithreaded, and short-lived helpers;
- observe rates within tolerance;
- change nice and restore original;
- SIGSTOP/SIGCONT a helper and verify control/PSTATE separation;
- simulate external nice drift;
- exit and rapidly create PIDs to exercise stale identity handling;
- group by PID/mount/network namespace inode;
- verify inactive follow target rebinding;
- verify snapshot target does not rebind;
- test cgroup v2 freeze only when delegated;
- test hidden/permission-denied proc fields;
- connect two clients to one daemon and verify shared sampling.

Never run control integration tests against arbitrary host processes.

### 26.4 Golden tests

Keep golden output for:

- narrow and wide tables;
- group trees;
- inactive managed view;
- JSON schema v1;
- capability reports;
- effective config dumps in all three encodings.

Normalize timestamps, PIDs, boot IDs, and terminal color codes.

### 26.5 Performance tests

Provide synthetic procfs fixtures for 100, 1,000, and 10,000 processes and
benchmark:

- identity scan;
- status/stat/io parsing;
- grouping/aggregation;
- selector evaluation;
- JSON and table rendering;
- state reconciliation with many rules.

## 27. Delivery plan

### Phase 0 — skeleton and contracts

- Go module, command skeleton, version/build info.
- Canonical config types and strict TOML/YAML/JSON adapters.
- Metric/dimension/column registries.
- Versioned JSON schema draft.
- Fake clock, procfs, controller, and state interfaces.

Exit criterion: config equivalence and strictness tests pass; `config check`,
`convert`, and `dump --effective` work.

### Phase 1 — useful read-only core

- Safe procfs inventory with PID/starttime identity.
- Light process metrics and rate sampler.
- Entity selector DSL.
- Arbitrary grouping pipeline, aggregation, leaves, having, sort, columns.
- `pm ps`, `pm stat`, JSON/NDJSON/table output.
- Built-in presets and capability reporting.

Exit criterion: the tool is already useful as a grouped `ps`/`pidstat` and meets
light-profile performance budgets.

### Phase 2 — runtime management and basic control

- Runtime state, locking, boot checks, inactive logical targets.
- Manual snapshot/follow targets.
- Nice and SIGSTOP/SIGCONT controller with safeguards.
- Original/desired/observed state, drift reporting, restore, unmanage.
- Managed view and inspect command.

Exit criterion: PID reuse tests prove stale actions are refused; nice restore
and stopped-process ambiguity are handled safely.

### Phase 3 — TUI

- Tree/table browser, grouping/leaf editor, sorting, filtering, column/metric
  picker.
- Managed panel, detail view, control preview and confirmation.
- Responsive independent sampling/render loop.

Exit criterion: every TUI query can be reproduced by printed effective CLI
arguments or an exported preset.

### Phase 4 — per-user daemon and persistent policies

- Unix socket protocol and embedded/auto/required modes.
- Shared collector engine.
- Config managed selectors and continuous reconciliation.
- User systemd unit generation/install helpers.
- Safe reload behavior.

Exit criterion: policy matches new processes after daemon start/reboot and two
clients share one collector schedule.

### Phase 5 — extended collectors and controllers

- FD/socket classification.
- cgroup v2 existing-group freeze/thaw, CPU, and I/O controls.
- Optional eBPF wakeup/network/syscall collectors.
- Optional perf counters.
- Optional persistent audit/history.

Each capability must be individually shippable and must not increase default
light-profile work when disabled.

## 28. MVP acceptance criteria

The MVP is Phases 0–2 and is complete when all are true:

1. `pm ps --group-by comm --leaf none --sort cpu:desc` reports correct aggregate
   CPU without double counting process memory.
2. `pm ps --group-by none --leaf process` produces a flat process list.
3. `pm stat 1s --count 3` prints three timestamped, append-only sample batches.
4. TOML, YAML, and JSON versions of the same configuration resolve to identical
   canonical config.
5. Unknown config keys and duplicate keys fail in all three formats.
6. Profiles plus granular `+metric`/`-metric` overrides work predictably.
7. `select` and `having` are parsed, type-checked, and evaluated at their
   documented stages.
8. PID identity includes starttime and stale PID actions are refused.
9. A process selected into runtime managed state records managed membership.
10. Nice control records original, desired, and observed values and can restore
    the original when permissions permit.
11. External nice change produces drift instead of silently rewriting history.
12. SIGSTOP intent is shown separately from observed `PSTATE`; pre-stopped tasks
    are not blindly continued on restore.
13. A disappeared PID is removed from the process view immediately while its
    logical follow-mode managed target remains `INACTIVE`.
14. Runtime state is private, atomically written, boot-scoped, and overrideable
    by explicit state directory.
15. Missing permissions/capabilities render unavailable values and explanations,
    not misleading zeroes.
16. Light profile meets the initial overhead budget on the documented benchmark
    host.
17. JSON schema is versioned and covered by golden tests.

## 29. Definition of done for each feature

A feature is not done until it includes:

- canonical model/registry metadata;
- CLI and config representation where applicable;
- capability/permission behavior;
- unavailable and race semantics;
- human and machine rendering;
- unit/integration tests;
- help text and at least one example;
- no additional work in disabled collectors;
- state migration handling if persisted data changes.

## 30. Open questions requiring an explicit later decision

These are intentionally not blockers for Phases 0–2:

1. Final project/binary name; `pm` remains provisional.
2. Exact app-ID resolver precedence across desktop environments and Flatpak/Snap.
3. Whether canonical disk metric names are long
   (`disk-read-bytes`) or interactive (`disk-rbps`).
4. TUI framework choice. Bubble Tea, tcell, or another library is an
   implementation decision after a small rendering benchmark.
5. eBPF implementation and distribution strategy: embedded object, CO-RE,
   optional companion, or build tag.
6. Exact wakeup attribution names and semantics.
7. Persistent history storage format and retention.
8. Whether a narrowly privileged helper is ever justified for cross-user/cgroup
   operations. It is excluded until a separate security RFC.
9. Dynamic metric-triggered control policies with hysteresis. They require a
   separate policy RFC.
10. Automatic process migration into `pm`-owned cgroups. It requires explicit
    systemd/container interaction rules.

## 31. Implementation guidance for an autonomous coding agent

The coding agent should:

1. implement phases in order and keep every phase runnable;
2. begin with interfaces, registries, and tests rather than the TUI;
3. use a fake procfs root and fake monotonic clock from the first collector;
4. avoid eBPF, cgroup migration, and perf until the light pipeline and safe
   control state are complete;
5. never key cached or controlled state by numeric PID alone;
6. never treat a missing metric as zero;
7. keep config formats semantically identical through one canonical DTO;
8. add dependencies conservatively and document why each is needed;
9. run `go test ./...`, `go test -race ./...`, fuzz smoke tests, `go vet`, and a
   chosen static analyzer in CI;
10. present implementation deviations as proposed RFC amendments rather than
    silently changing semantics.

Suggested first pull-request sequence:

```text
PR 1  module, config adapters, canonical model, registries, CLI skeleton
PR 2  procfs identities and safe parsers with fixtures/fuzzing
PR 3  sampler and light metrics
PR 4  selector DSL
PR 5  grouping/aggregation/sort/projection
PR 6  ps/stat and JSON/table renderers
PR 7  runtime state and managed targets
PR 8  nice controller, drift, restore
PR 9  STOP/CONT controller and safeguards
PR 10 TUI
PR 11 daemon and config policies
```

## 32. Summary of normative decisions

- Linux-only, Go reference implementation.
- `pm` is a replaceable working name.
- Collection/query and control/state are separate planes.
- PID plus starttime and boot scope define instance identity.
- Grouping is an ordered list of dimensions; leaves are independently
  process/thread/none.
- `select` is pre-group; `having` is post-aggregation.
- Metrics are registry/capability driven with independent collector intervals.
- Default profile is inexpensive and does not require root/eBPF.
- TOML, YAML, and JSON are first-class, strict, semantically identical configs.
- Config intent, runtime state, and optional history use separate XDG locations.
- Persistent managed selectors express membership; control policy is optional.
- Continuous enforcement requires the optional per-user daemon.
- Original, desired, and observed control values are all retained.
- Restore and unmanage are different operations.
- Drift is explicit; default behavior reports rather than fights external tools.
- Vanished PID/TID rows disappear, while logical managed targets can remain
  inactive and later rebind in follow mode.
- Missing/unsupported/permission-denied data is never represented as zero.

