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
   `make check` is green (fmt/vet/lint/test/race/cover 81.5%).

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

_Last updated by the autonomous build session._
