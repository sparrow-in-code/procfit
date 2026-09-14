# Open questions / decisions taken while you were AFK

I did not stop for these — I made a reasonable call, recorded it here, and kept
going. Override any of them and I'll adjust.

## Decided (with rationale) — change if you disagree

1. **Project/binary name → `procfit`** (as requested). The binary name lives in a
   single constant (`internal/meta`) so it stays trivially changeable (RFC §13).
2. **Go module path → `github.com/netikras/procfit`.** No git remote exists yet;
   this is the conventional, remote-ready path. Trivial to rename with a
   find/replace if you want something else.
3. **Repository *directory* left as `.../projects/pm`.** Renaming the working
   directory mid-session risks breaking the tooling/harness path. The module,
   binary, docs, and RFC are all `procfit`; only the folder name is still `pm`.
   Rename the folder yourself when convenient, or tell me to.
4. **Ticket IDs kept as `PM-####`.** Renaming every ticket file + every
   `depends:` reference is churn with no functional value; treat `PM` as the
   ticket-tracker prefix, decoupled from the product name. Say the word and I'll
   rebrand to `PF-####`.
5. **Canonical disk metric ids → `disk-rbps` / `disk-wbps`** (RFC §30.3 rec.),
   with `disk-read-bytes` / `disk-write-bytes` as aliases.
6. **Toolchain provisioned via nix** (not preinstalled on the box):
   `nix profile add nixpkgs#go nixpkgs#gnumake nixpkgs#gcc nixpkgs#golangci-lint`
   (go 1.26.7, golangci-lint 2.x). Put them on PATH per shell:
   `export PATH="$HOME/.nix-profile/bin:$PATH"`. Race tests need `CC=gcc`.
   `make check` is green (fmt/vet/lint/test/race/cover ~80%).
7. **`name` = clean program basename.** `DisplayName`/`name` now takes the exe
   token's basename even when argv[0] carries jammed args (Chrome writes its whole
   command line into argv[0]). This makes `--group-by name` group all chrome
   processes together; the full command is available via the new `cmdline` column.
   `displayNameCap` raised 64→256 so `--target-width` can widen names.
8. **Leaf identifier is its own column.** Default columns now include `pid`
   (blank on group rows), and `--leaf thread` additionally shows `tid`, so a
   single process is never mistaken for an aggregate.
9. **Config precedence is layered + reorderable.** Resolution is
   `default < config < env < args` per setting, recorded with its winning source;
   env vars are `PROCFIT_<SETTING>`; `--config-precedence` reorders the layers
   (`default` is always the floor); `--show-config` prints the resolved table.
10. **TUI leaf mode is independent of grouping.** `g` cycles group-by only; new
    `t` cycles leaf (process/thread/none). Pause-refresh is `p`/Space.

## Genuinely want your input (non-blocking; I proceed with the default)

- **A. Cross-platform in the RFC.** You asked for portability as a global pattern
  (ports/adapters), but RFC §32 still says "Linux-only". Default: I keep Linux as
  the only *shipped* adapter and treat portability as architecture only, per
  PM-9003. Want me to formally amend RFC §4/§32 to declare cross-platform an
  intended future direction?
- **B. TUI framework** (Phase 3). Default per PM-0301: benchmark tcell vs Bubble
  Tea when I reach Phase 3; leaning tcell for a dense, high-refresh table UI.
- **C. Folder rename** (see decided #3).
- **D. eBPF & perf — RESOLVED using the root Alma box you provided.**
  - **eBPF `wakeups` (PM-0503): DONE and validated live as root** — real
    per-process wakeup rates. BPF object is compiled with clang and embedded
    (`make bpf` regenerates it); loaded via pure-Go cilium/ebpf, so the target
    needs no toolchain.
  - **perf (PM-0504): implemented + validated**, but the test box is a KVM VM
    with **no virtual PMU** (`perf_event_open` HW cpu-cycles → ENOENT; SW
    task-clock → OK), so hardware counter *values* need a **bare-metal or
    vPMU-enabled host**. On a suitable host it should work unchanged; point me at
    one to capture real numbers. Remaining event metrics (`timer-wakeups`,
    `net-*`) still need dedicated BPF programs.

- **E. PM-0303 (TUI managed control) was marked DONE but never implemented** — no
  control code existed in `internal/tui/`, and acceptance criteria were unchecked.
  I reopened it and am implementing TUI control (nice/stop/continue/freeze/
  restore) reusing the Phase-2 controllers, with the §19.3 preview + confirmation.
- **F. Space keybinding.** The CLI help advertised `--stop … (Space in TUI)`, but
  I had used Space for pause-refresh. Default: **pause is `p` (and Space)** for
  now; once TUI control lands, Space becomes stop/continue and pause stays on `p`.
  Say if you'd rather keep Space as pause permanently.
- **G. Default grouping stays flat (ps-like).** `--group-by none` is the default;
  hierarchy is opt-in via `g`/`--group-by`. Tell me if you want a grouped default.
- **H. `--collapse-groups` is a shared view knob that only the TUI acts on.** It
  starts the TUI with every group folded (also `collapse_groups` in config,
  `PROCFIT_COLLAPSE_GROUPS` env; `c`/`C` fold/unfold all live). I wired it through
  the shared query-flags + config precedence machinery (like `target-width`) for a
  consistent, testable path, so `ps`/`stat` *accept but ignore* it today. If you'd
  rather it be TUI-only at the CLI surface (rejected by `ps`), or have `ps` honour
  it as a "group summary" view (print only group rows, hide leaves), say so.

_Last updated by the autonomous build session._
