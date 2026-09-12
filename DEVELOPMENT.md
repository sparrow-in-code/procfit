# DEVELOPMENT.md — How we build `pm`

This is the engineering handbook for `pm`, a low-overhead Linux process observer
and workload controller (Go). It defines the architecture, the coding and
testing discipline, and the workflow every contributor — human or agent — must
follow.

> **Read these first, in order:**
> 1. [`RFC-pm-linux-process-observer-controller.md`](RFC-pm-linux-process-observer-controller.md) — the product spec and single source of truth.
> 2. This file (`DEVELOPMENT.md`) — how we build it.
> 3. [`rfc/README.md`](rfc/README.md) — the ticket/work-tracking system.
> 4. Whatever ticket is in [`rfc/wip/`](rfc/wip/) — the current task.
>
> When this file and the RFC disagree on *product* behaviour, the RFC wins.
> When they disagree on *process*, this file wins. Resolve real conflicts with a
> `PM-90NN` amendment ticket — never a silent deviation (RFC §31.10).

---

## 1. What we are building (one screen)

`pm` normalizes Linux processes/threads into a stable model, then runs a single
query pipeline that powers a TUI, a `ps`-like table, a `vmstat`-like stream, and
JSON/NDJSON output. A **separate** control plane manages workloads (nice,
stop/continue, freeze, …) while preserving original/desired/observed state for
safe restore and drift detection.

Two planes, never entangled (RFC §6.1–6.2):

```text
OBSERVATION PLANE (read)
collectors → normalized entities+samples → select → group → aggregate
          → leaves → having → sort → project → renderer

CONTROL PLANE (side, write)
target/selection → resolve bindings → revalidate identity → controller
                → record original/desired/observed → reconcile → detect drift
```

Query rows may be *annotated* from control state, but observation must keep
working when control is unavailable.

---

## 2. Architecture principles

### 2.1 The non-negotiable invariants (RFC §6, §32)

These are correctness, not style. Violating one is a bug even if tests pass:

1. **Numeric PID is never identity.** Identity is
   `boot_id + pid_namespace_inode + pid + starttime`. Revalidate immediately
   before any control action. Never key cached or controlled state by PID alone.
   (§6.3, §21.2, §31.5)
2. **Unknown is not zero.** Every sample carries availability + quality. Missing
   data renders `-`/`?` and keeps its reason in JSON. Never coerce missing → 0.
   (§6.4, §31.6)
3. **Renderers are dumb.** No renderer implements collection, filtering,
   grouping, or aggregate semantics. (§6.1)
4. **Control is a side plane.** It is never embedded in the query pipeline and
   never a precondition for observation. (§6.2)
5. **`select` is pre-group (per entity); `having` is post-aggregation (per
   row).** They use different evaluators. (§10.1)
6. **Config is code-like intent.** Strict parsing; three formats
   (TOML/YAML/JSON) with *identical* semantics through one canonical DTO. (§9)
7. **Managed membership ≠ control.** Retaining a target for visibility applies
   nothing. (§6.6)
8. **Restore ≠ unmanage.** Distinct operations with distinct guarantees. (§15.8)
9. **Drift is reported, not fought, by default.** We do not silently overwrite
   another actor's change. (§15.7)
10. **Never setuid; never require root by default.** Degrade by capability with
    explicit reasons. (§20)

### 2.2 SOLID, applied to this codebase

- **Single Responsibility.** Each package in the layout (§4) owns one concern:
  `procfs` reads, `collect` schedules, `query` transforms, `render` presents,
  `control` mutates, `state` persists. No `util` dumping ground (RFC §24).
- **Open/Closed.** Growth happens by *registration*, not editing switch
  statements:
  - a new metric = a new `MetricDescriptor` + collector;
  - a new column = a new column descriptor;
  - a new renderer/collector/controller = one interface implementation + a
    registry entry.
  Adding these must not touch the query engine.
- **Liskov.** Every collector, resolver, controller, renderer, and decoder is
  substitutable behind its interface. Fakes (`internal/testutil`) are first-class
  substitutes and must satisfy the same contracts as production types.
- **Interface Segregation.** Small interfaces (`Collector`, `Resolver`,
  `Controller`, `Engine`, `Decoder`). The TUI/CLI depend on the client-facing
  `Engine`/`Controller` interfaces (RFC §25), never on concrete daemon or
  collector packages.
- **Dependency Inversion.** High-level policy (query, control planning) depends
  on abstractions. Linux syscalls, the clock, procfs, and IPC transport sit
  behind interfaces so they can be faked. The embedded engine and the daemon-
  over-IPC client implement the *same* interfaces (RFC §17, §25).

### 2.3 Clean-code rules we actually enforce

- Prefer composition over inheritance; decorate entities via `Resolver`s.
- Functions do one thing; keep cyclomatic complexity low; extract parsers.
- Names describe intent; exported identifiers have doc comments (`go doc` clean).
- Errors are wrapped with context (`fmt.Errorf("...: %w", err)`); expected
  outcomes (process vanished) are not errors to log per-occurrence (§21.1).
- No global mutable state except registries populated at `init` and never
  mutated afterward.
- No direct `time.Now()` / `/proc` access in business logic — go through the
  injected clock / procfs reader (enables TDD and determinism).
- Concurrency: immutable sample generations; share by communicating; everything
  concurrent is exercised under `-race`.

### 2.4 Dependency policy (RFC §31.8)

Add dependencies conservatively. Each new module dependency must be justified in
the PR/ticket ("why not stdlib?"). Prefer the standard library for parsing,
concurrency, and I/O. The TUI framework is the one deliberately large dependency
and is chosen via benchmark in `PM-0301`.

---

## 3. Test discipline (TDD first)

We practise **test-driven development**: red → green → refactor. Write the
failing test that expresses the acceptance criterion, make it pass minimally,
then refactor with the test as a safety net. The Phase 0 test seams
(`PM-0004`: fake clock, fake procfs, fake controller, fake state) exist
specifically so TDD is possible from the first collector (RFC §31.3).

### 3.1 Test categories (RFC §26)

| Category | Purpose | Runs in CI |
|---|---|---|
| **Unit** | Logic in isolation using fakes. The default. | always |
| **Property/Fuzz** | Parsers/decoders never panic; invariants hold (grouping preserves identity set; short-circuit respected). | fuzz smoke always; longer fuzz on demand |
| **Integration** | Real `/proc`, real syscalls, spawned helper processes in a disposable user namespace/container. Never against arbitrary host processes. | gated (build tag / label) |
| **Golden** | Stable output: tables, trees, JSON schema v1, capability reports, effective-config dumps. Normalize timestamps/PIDs/boot-ids/colors. | always |
| **Perf/Bench** | Synthetic procfs fixtures (100/1k/10k procs); enforce the §22 budgets. | benchmarks tracked; regressions reviewed |

### 3.2 Coverage expectations

- Core logic packages (`config`, `expr`, `query`, `control`, `state`,
  `procfs`, `metrics`) target **high** unit coverage — these are pure and
  fully fakeable, so low coverage there is a smell.
- Every RFC §28 MVP criterion and every ticket acceptance criterion must map to
  at least one test.
- Every bug fix starts with a failing regression test.

### 3.3 Running tests

```bash
go build ./...
go test ./...
go test -race ./...
go vet ./...
staticcheck ./...          # or the chosen analyzer
go test -run TestGolden ./... -update   # regenerate golden files (review diffs!)
go test -tags integration ./...         # gated, needs a suitable environment
```

CI (`PM-9001`) runs build, test, `-race`, fuzz smoke, `vet`, static analysis,
gofmt/goimports, and golden verification on every PR. Merges are blocked on
failure.

---

## 4. Repository layout (RFC §24)

```text
cmd/pm/                   entry point
internal/app/             command orchestration / routing
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
rfc/                      tickets (backlog/wip/archive) — see rfc/README.md
```

Keep Linux syscalls behind small interfaces. No package named `util`.

---

## 5. Work workflow (how a change happens)

1. **Pick a ticket.** Take the highest-priority unblocked ticket from
   `rfc/backlog/` (dependencies must be in `rfc/archive/`). If nothing fits,
   write a new ticket first.
2. **Start it.** `git mv` the ticket to `rfc/wip/`, set `state: WIP` and `owner`.
   Only one ticket per owner in `wip/` at a time.
3. **TDD the acceptance criteria.** Write failing tests → implement → refactor.
4. **Keep docs in lockstep** (§6). Update affected `.md` files in the same
   change.
5. **Run the full local gate** (§3.3).
6. **Finish it.** When every acceptance criterion + the Definition of Done (§7)
   is met, `git mv` the ticket to `rfc/archive/`, set `state: DONE`, and record
   the commit/PR + date at the bottom of the ticket.
7. **Spin off follow-ups** as new `backlog/` tickets — never silent scope creep.

Follow the RFC's phase order (§27) and PR sequence (§31); keep every phase
runnable. Do not jump to eBPF/cgroup-migration/perf before the light pipeline
and safe control state exist (RFC §31.4).

### Commit / PR conventions

- Small, reviewable commits; imperative subject; reference the ticket id
  (e.g. `query: stable multi-key sort (PM-0106)`).
- A PR maps to one ticket (mirror the RFC §31 PR sequence where practical).
- CI green is mandatory before merge.

---

## 6. Documentation policy (keep `.md` current)

Docs are part of "done", not an afterthought:

- **Every feature updates its docs in the same change.** Help text, examples,
  and the relevant file under `docs/` must reflect reality (RFC §29).
- **JSON output is a versioned public interface** (RFC §7.5, §18.4). Any change
  bumps/annotates the schema and updates `docs/json-schema.md` + golden tests.
- **The master RFC is authoritative.** Approved deviations are folded back into
  it via a `PM-90NN` amendment ticket; do not let code and RFC silently diverge.
- **Config examples** (TOML/YAML/JSON) are generated from the canonical model in
  documentation tests, not hand-maintained (RFC §9.5).
- `AGENTS.md`/`CLAUDE.md` (symlink), this file, and `rfc/README.md` describe the
  process; update them when the process changes.

---

## 7. Definition of Done (RFC §29)

A feature/ticket is **not done** until all apply:

- [ ] Canonical model / registry metadata added where relevant.
- [ ] CLI and config representation where applicable.
- [ ] Capability / permission behaviour implemented and reported.
- [ ] Unavailable and race semantics handled (unknown ≠ zero; vanish is normal).
- [ ] Human and machine rendering both correct.
- [ ] Unit tests (+ property/integration/golden as applicable) green, including
      `-race`.
- [ ] Help text and at least one example.
- [ ] No additional work performed by disabled collectors.
- [ ] State migration handled if persisted data changed.
- [ ] Docs updated (§6); ticket acceptance criteria all checked.

---

## 8. Roadmap (phases → tickets)

Detailed, dependency-ordered tickets live in `rfc/`. Summary (RFC §27):

| Phase | Theme | Tickets |
|---|---|---|
| 0 | Skeleton & contracts | PM-0001..0005, PM-9001, PM-9002 |
| 1 | Useful read-only core | PM-0101..0109 |
| 2 | Runtime management & basic control (**MVP = Phases 0–2**) | PM-0201..0205 |
| 3 | TUI | PM-0301..0304 |
| 4 | Per-user daemon & persistent policies | PM-0401..0404 |
| 5 | Extended collectors & controllers | PM-0501..0505 |

**MVP acceptance** is the full RFC §28 checklist; verify it before declaring
Phases 0–2 complete.
