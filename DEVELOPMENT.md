# DEVELOPMENT.md — How we build `procfit`

This is the engineering handbook for `procfit`, a low-overhead Linux process observer
and workload controller (Go). It defines the architecture, the coding and
testing discipline, and the workflow every contributor — human or agent — must
follow.

> **Read these first, in order:**
> 1. [`RFC-procfit-linux-process-observer-controller.md`](RFC-procfit-linux-process-observer-controller.md) — the product spec and single source of truth.
> 2. This file (`DEVELOPMENT.md`) — how we build it.
> 3. [`rfc/README.md`](rfc/README.md) — the ticket/work-tracking system.
> 4. Whatever ticket is in [`rfc/wip/`](rfc/wip/) — the current task.
>
> When this file and the RFC disagree on *product* behaviour, the RFC wins.
> When they disagree on *process*, this file wins. Resolve real conflicts with a
> `PM-90NN` amendment ticket — never a silent deviation (RFC §31.10).

---

## 0. Prime directives (read twice)

These five are **not aspirational** — they are merge-blocking acceptance
criteria for every single change. A PR that ships a feature but breaks one of
these is *not done* and must not be merged. Full detail follows in §2 and §3;
this is the short, loud version.

1. **SOLID, always.** Every type has one reason to change; every extension point
   is an interface; we depend on abstractions, not concretions. If you cannot
   name which SOLID principle a new type serves, stop and redesign. See §2.2.
2. **Design-pattern-driven extensibility, everywhere.** This codebase *will* be
   extended, overridden, and swapped at many layers — new metrics, collectors,
   resolvers, controllers, renderers, decoders, transports, config/output formats,
   **and whole new operating-system backends (Windows, macOS, other Unix/BSD)**.
   Build those seams with explicit, named patterns (Strategy, Registry/Factory,
   Decorator, Adapter, Facade, Template Method, Observer, …) inside a
   **ports-and-adapters** core. Adding a variant — including a new OS — must mean
   *adding an adapter package + registering it*, **never editing core logic or a
   central switch**. See §2.3 and §2.4.
3. **Clean code, capped size.** Small, single-purpose, well-named units. The hard
   size caps in §2.6 are enforced, not suggested. Big files/structs/functions are
   a design failure, not a formatting nit.
4. **≥ 80% test coverage, TDD first.** 80% is the **floor**, enforced in CI as a
   hard gate (§3.2). Core logic packages are expected far higher. Write the
   failing test before the code. Coverage below 80% fails the build the same way a
   compile error does.
5. **Docs move with code.** The `.md` for a feature changes in the same commit as
   the feature (§6).

> Bring the discipline. Program to interfaces, favour composition, use the
> Gang-of-Four patterns where they fit, and keep structs small and cohesive. Go's
> idioms have their own spelling (small interfaces, embedding, functional options)
> but the design intent is universal — §2.3 maps the patterns to their Go form so
> we stay idiomatic *and* disciplined.

---

## 1. What we are building (one screen)

`procfit` normalizes Linux processes/threads into a stable model, then runs a single
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

### 2.2 SOLID, applied to this codebase (mandatory)

SOLID is the first thing a reviewer checks. Each principle below has a concrete
"what this means here" and a "you are violating it if…" test.

- **S — Single Responsibility.** Each package in the layout (§4) owns exactly one
  concern: `procfs` reads, `collect` schedules, `query` transforms, `render`
  presents, `control` mutates, `state` persists. Same at the type level: a struct
  has one reason to change. No `util` dumping ground (RFC §24).
  *Violating it if:* a type both parses and renders; a function both decides and
  performs I/O; you reach for a "misc"/"helpers"/"common" package.
- **O — Open/Closed.** Growth happens by *adding a type and registering it*, never
  by editing a `switch`/`if-else` ladder over a kind:
  - a new metric = a new `MetricDescriptor` + collector;
  - a new column = a new column descriptor;
  - a new renderer / collector / controller / resolver / decoder / transport =
    one interface implementation + one registry entry.
  Adding any of these must touch **zero** existing query/render/control logic.
  *Violating it if:* supporting a new variant means editing a central switch, or a
  core package imports a concrete variant.
- **L — Liskov Substitution.** Every collector, resolver, controller, renderer,
  and decoder is fully substitutable behind its interface — including the fakes in
  `internal/testutil`, which must honour the same contract (same error/availability
  semantics) as production types. A test passing against the fake but failing
  against the real type means the contract is under-specified — fix the contract.
- **I — Interface Segregation.** Interfaces stay small and role-specific
  (`Collector`, `Resolver`, `Controller`, `Engine`, `Decoder`, `Renderer`). No
  "fat" interface that forces implementers to stub methods they don't use. The
  TUI/CLI depend on the client-facing `Engine`/`Controller` interfaces (RFC §25),
  never on concrete daemon or collector packages.
- **D — Dependency Inversion.** High-level policy (query, control planning)
  depends only on abstractions. Linux syscalls, the clock, procfs, and the IPC
  transport all sit behind interfaces defined by the *consumer* and injected in.
  The embedded engine and the daemon-over-IPC client implement the *same*
  interfaces (RFC §17, §25). *Violating it if:* a business-logic package imports
  `os`, `syscall`, or a concrete transport directly.

**Dependency direction rule:** dependencies point inward toward `model`/policy.
`model` imports nothing project-specific. Renderers, transports, and syscall
adapters are the outermost ring and may be swapped without touching the core.

### 2.3 Design patterns we use deliberately (pattern → Go form)

This codebase is explicitly built to be extended and overridden at many layers.
Use these named patterns for those seams; a reviewer may ask "which pattern is
this seam?" and "how do I add a variant without editing existing code?".

| Need / seam | Pattern | Go form in `procfit` |
|---|---|---|
| Swap algorithm/behaviour behind a stable contract | **Strategy** | `Collector`, `Controller`, `Renderer`, `Resolver`, `Decoder` interfaces |
| Discover/construct variants by id without a switch | **Registry + Factory** | metric/dimension/column registries; collector/controller registration at `init` |
| Enrich an entity without subclassing | **Decorator** | `Resolver` chain decorating `*Process` |
| Uniform API over TOML/YAML/JSON differences | **Adapter** | per-format `Decoder` → one canonical DTO |
| Hide subsystem complexity from clients | **Facade** | `internal/client` engine facade; `internal/app` command orchestration |
| Fixed pipeline skeleton, pluggable steps | **Template Method / Pipeline** | the query pipeline stages (§1); collector `Collect` skeleton |
| Notify subscribers of new snapshots | **Observer / Pub-Sub** | `Engine.Subscribe` snapshot stream; daemon events |
| Configurable construction without telescoping ctors | **Functional options** | `New(...Option)` constructors |
| One behaviour, many concrete kinds | **Interpreter / Visitor** | `expr` AST evaluation and type-checking |
| Encapsulate a mutation as a reversible unit | **Command + Memento** | control `Plan`/`Apply`/`Restore` with captured original state |

Guidance: **program to interfaces, favour composition over inheritance** (Go has
no inheritance — use embedding + interfaces), and prefer a small interface + many
implementations over one type with mode flags. Do not add a pattern where a plain
function suffices; do not hand-roll a switch where the Registry seam already
exists.

### 2.4 Extensibility & platform portability are global requirements

Extensibility is not a per-feature nicety here — it is the **default posture of
the whole codebase**, expressed as a **ports-and-adapters (hexagonal)**
architecture.

```text
                 ┌─────────────────────────────────────────────┐
   adapters →    │  OS-AGNOSTIC CORE                            │   ← adapters
 (outer ring)    │  model · expr · query · control-planning ·  │  (outer ring)
                 │  config-semantics · metric/dim/column        │
  Linux procfs   │  registries · state logic                   │  table/json/tui
  Windows API    │                                             │  renderers
  macOS/BSD      │  defines PORTS (interfaces), depends on      │  TOML/YAML/JSON
  eBPF/perf      │  nothing platform-specific                  │  decoders
  cgroup ctl     └─────────────────────────────────────────────┘  IPC transports
```

**The core is OS-agnostic and never imports OS-specific packages.** Nothing in
`model`, `expr`, `query`, `control` planning, config *semantics*, or the
registries may import `syscall`/`golang.org/x/sys`, assume `/proc`, or reference
a specific OS concept. All platform specifics live behind **ports** — interfaces
owned by the core — implemented by **adapter packages** in the outer ring.

Representative ports (names illustrative, contracts normative):

```text
ProcessSource     enumerate + read normalized entities/identity for this OS
CapabilityProbe   report what this platform/build/privilege level supports
Controller        nice / suspend-resume / freeze / priority per platform
SignalSender      deliver signals/terminations by validated identity
GroupController    cgroup-like / job-object-like resource control
Clock, Transport, Renderer, Decoder, Collector, Resolver, ...
```

Rules that make new backends *additive, not invasive*:

1. **Linux is the first shipped adapter, not a baked-in assumption.** RFC §32's
   "Linux-only" is a **shipping-scope** decision (what we release now), *not* an
   architectural one. Adding Windows/macOS/BSD/other-Unix later must be a new
   adapter package (e.g. `internal/procfs` ↔ a sibling `internal/procsrc/<os>`)
   implementing the same ports — **zero changes to the core**. If any real
   conflict with RFC scope surfaces, raise a `PM-90NN` amendment (tracked by
   `PM-9003`); do not smear OS specifics into the core.
2. **Isolate OS specifics** behind build-tagged files (`*_linux.go`,
   `*_windows.go`, `*_darwin.go`, `*_bsd.go`) and/or separate adapter packages,
   selected by a factory/registry at startup. Shared behaviour stays in tag-free
   files.
3. **Same global pattern for every extension axis**, not just OS: metrics,
   collectors, resolvers, controllers, renderers, output formats, config formats,
   IPC transports, target kinds, dimensions, columns. Each is a **port + a
   registry**; adding support = writing an adapter and registering it.
4. **Graceful degradation is built in.** The capability model + "unknown ≠ zero"
   (§2.1) means a port with no adapter on the current platform reports
   `unsupported`, and inherently-Linux concepts (namespaces, cgroups) become
   optional capabilities that render unavailable elsewhere — never fabricated.
5. **Keep the vocabulary neutral.** Prefer OS-neutral names in the core model;
   where a concept is intrinsically platform-specific, model it as an optional,
   capability-gated attribute behind a port rather than a mandatory field.

*You are violating this section if:* a core package imports `x/sys` or hardcodes
`/proc`; a new capability requires editing a switch in the core; or supporting a
second OS would require touching anything but new adapter packages and their
registration.

### 2.5 Clean-code rules we actually enforce

- Prefer composition over inheritance; decorate entities via `Resolver`s.
- Functions do one thing; keep cyclomatic complexity low; extract parsers.
- Names describe intent; exported identifiers have doc comments (`go doc` clean).
- Errors are wrapped with context (`fmt.Errorf("...: %w", err)`); expected
  outcomes (process vanished) are not errors to log per-occurrence (§21.1).
- No global mutable state except registries populated at `init` and never
  mutated afterward.
- No direct `time.Now()` / `/proc` / `syscall` access in business logic — go
  through the injected clock / procfs reader / controller (enables TDD and
  determinism, and satisfies DIP).
- Concurrency: immutable sample generations; share by communicating; everything
  concurrent is exercised under `-race`.

### 2.6 Size caps (hard limits, enforced in review)

Small units are how SOLID stays real. These are **caps, not targets** — most code
should be well under them. Exceeding a cap is a signal to split, and requires an
explicit, justified `//nolint`-style note in the PR *and* reviewer sign-off.

| Unit | Soft target | **Hard cap** |
|---|---|---|
| Function / method body | ≤ 40 lines | **60 lines** |
| Function parameters | ≤ 4 | **5** (beyond → options struct) |
| Cyclomatic complexity / function | ≤ 8 | **12** |
| Struct fields | ≤ 10 | **15** (beyond → compose sub-structs) |
| Interface methods | ≤ 4 | **6** (beyond → segregate, see ISP) |
| Source file | ≤ 400 lines | **600 lines** |
| Nesting depth | ≤ 3 | **4** (use early returns/guard clauses) |
| Package exported surface | keep minimal | review if it sprawls |

If a unit wants to exceed a cap, the correct move is almost always to extract a
type or function — which usually surfaces a missing abstraction anyway. Linters
(`funlen`, `gocyclo`, `gocognit`, `nestif`, `lll`) enforce these in CI (`PM-9001`).

### 2.7 Dependency policy (RFC §31.8)

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

### 3.2 Coverage — ≥ 80% is a hard gate

- **≥ 80% statement coverage is the floor for the whole module and is enforced in
  CI (`PM-9001`).** A PR that drops total coverage below 80%, or that adds code
  materially below 80% for its package, **fails the build** — same severity as a
  compile error. This is not negotiable per the project owner.
- Core logic packages (`config`, `expr`, `query`, `control`, `state`, `procfs`,
  `metrics`, `collect`) are pure and fully fakeable — they are expected **well
  above** the floor (target ≥ 90%). Low coverage there is a design smell, usually
  a missing seam.
- Coverage is a floor, not the goal: tests must assert behaviour and invariants,
  not merely execute lines. Every RFC §28 MVP criterion and every ticket
  acceptance criterion maps to at least one test.
- Every bug fix starts with a failing regression test.
- Exclusions are narrow and explicit (e.g. generated code, thin `main`); they are
  listed in the coverage config and reviewed, never ad hoc.

### 3.3 Running tests

```bash
go build ./...
go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out   # must be ≥ 80%
go vet ./...
staticcheck ./...          # or the chosen analyzer
golangci-lint run          # incl. funlen/gocyclo/gocognit/nestif (size caps §2.6)
go test -run TestGolden ./... -update   # regenerate golden files (review diffs!)
go test -tags integration ./...         # gated, needs a suitable environment
```

CI (`PM-9001`) runs build, test, `-race`, **coverage (≥ 80% hard gate)**, fuzz
smoke, `vet`, static analysis, size-cap linters, gofmt/goimports, and golden
verification on every PR. Merges are blocked on any failure.

---

## 4. Repository layout (RFC §24)

Read this as concentric rings (§2.4): `model` and the policy packages are the
**OS-agnostic core**; `procfs`, syscall adapters, transports, and renderers are
the **outer ring** implementing core-defined ports.

```text
cmd/procfit/                   entry point (thin)
internal/app/             command orchestration / routing (Facade)
internal/config/          adapters, canonical DTO, merge, normalize, validate
internal/expr/            lexer, parser, type checker, evaluator      ── core
internal/model/           entities, identities, metric/control values ── core (imports nothing OS-specific)
internal/ports/           port interfaces the core owns (ProcessSource, Controller, CapabilityProbe, ...)
internal/collect/         registry, scheduler, collectors             ── core policy over ports
internal/resolve/         app/systemd/cgroup/namespace label resolvers (Decorator chain)
internal/query/           select, grouping, aggregate, having, sort, projection ── core
internal/metrics/         metric descriptors and profiles (Registry)
internal/control/         planner, safeguards, restore/reconcile      ── core policy over Controller ports
internal/state/           runtime persistence, locks, history interface
internal/daemon/          engine lifecycle and IPC server
internal/client/          IPC and embedded-engine facade (Facade)
internal/render/table/    one-shot and streaming tables (Renderer adapter)
internal/render/json/     versioned machine formats (Renderer adapter)
internal/tui/             interactive UI
internal/testutil/        fake clock, proc fixtures, fake controllers (test adapters)
docs/                     user and architecture documentation
rfc/                      tickets (backlog/wip/archive) — see rfc/README.md
```

Platform adapters live behind the ports and are selected at startup by a
factory/registry. Today only the Linux adapter (`internal/procfs` + Linux
controllers) exists; a future OS is a **sibling adapter package** (e.g.
`internal/platform/<os>/`) implementing the same `internal/ports` interfaces —
**no core edits** (§2.4). Use build tags (`*_linux.go`, `*_windows.go`,
`*_darwin.go`, `*_bsd.go`) for OS-specific files.

Hard rules: the core (`model`, `expr`, `query`, `control` planning,
config-semantics, registries) must not import `syscall`/`x/sys` or assume `/proc`.
Keep OS syscalls behind small port interfaces. No package named `util`.

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

- [ ] **SOLID honoured** (§2.2): new variability sits behind an interface + registry
      (Open/Closed); no core `switch` edited to add a variant.
- [ ] **Extensibility/ports respected** (§2.4): no OS-specific import in the core;
      platform/tool specifics live behind ports; new backends would be additive.
- [ ] **Size caps met** (§2.6) or an explicitly justified, signed-off exception.
- [ ] **Coverage ≥ 80%** for changed code and module total not regressed (§3.2).
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
| 0 | Skeleton & contracts | PM-0001..0005, PM-9001, PM-9002, PM-9003 |
| 1 | Useful read-only core | PM-0101..0109 |
| 2 | Runtime management & basic control (**MVP = Phases 0–2**) | PM-0201..0205 |
| 3 | TUI | PM-0301..0304 |
| 4 | Per-user daemon & persistent policies | PM-0401..0404 |
| 5 | Extended collectors & controllers | PM-0501..0505 |

**MVP acceptance** is the full RFC §28 checklist; verify it before declaring
Phases 0–2 complete.

---

## 9. Releasing (GitHub Releases + GHCR)

Releases are **version-driven**: a `vX.Y.Z` either pushed as a git tag or entered
in the *Run workflow* dialog runs `.github/workflows/release.yml`, which:

1. **verify** — `go vet` + `go test ./...` (a broken tag never ships).
2. **binaries** — builds static, `CGO_ENABLED=0`, `-trimpath` binaries for
   `linux/amd64` and `linux/arm64`, stamps `internal/meta.{Version,Commit,BuildDate}`
   from the tag/commit, and packages `procfit_<tag>_linux_<arch>.tar.gz` + a
   `.sha256`.
3. **release** — publishes a **GitHub Release** for the tag via
   `softprops/action-gh-release`, attaching the tarballs + checksums with
   auto-generated notes. Runs on a tag push or a manual run.
4. **image** — builds and pushes a multi-arch **GHCR** image
   (`ghcr.io/<owner>/procfit`) tagged `:X.Y.Z`, `:X.Y`, and `:latest`, on the
   same conditions.

Two equivalent ways to cut a release — pick one:

```console
# A) push a tag (scriptable, the usual path)
git tag -a v1.2.3 -m "procfit v1.2.3"
git push origin v1.2.3
```

**B) the Actions UI** — open *Actions → release → Run workflow*, choose the
branch/commit, and enter the version (e.g. `v1.2.3`) in the **tag** input. This
runs the same pipeline and **creates the tag + Release at that commit**.

Both paths publish. The release *tag* names the artifacts; the version **embedded**
in the binary is SemVer plus a build-metadata id — `<core>+<yyyyMMdd-HHmmss>-<sha>`
(e.g. `1.2.3+20260922-154210-d60fa93`) — so a binary always reports exactly when
and from which commit it was built. `<core>` is the tag (`v`-stripped), or the
nearest tag / `0.1.0` for untagged local builds. `scripts/version.sh` is the single
source of truth for the shell paths (`make build`, the release job); `flake.nix`
composes the same from flake metadata, and an un-stamped `go build` reports
`0.1.0-dev`. Use SemVer tags; pre-releases like `v1.2.3-rc1` publish too (mark them
pre-release in the GitHub UI if desired).

> **Why might "publish GitHub release" show as _skipped_?** The publish + image
> **push** steps are guarded to a tag ref or a manual run. If you trigger the
> workflow some other way (or an old `workflow_dispatch` with no tag), `verify`
> and `binaries` still run but publishing is skipped by design. Use one of the two
> paths above to actually publish.

Permissions are declared in the workflow (`contents: write` for the Release,
`packages: write` for GHCR) and use the default `GITHUB_TOKEN`; no extra secrets
are required.
