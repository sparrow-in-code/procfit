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
- **D. eBPF (PM-0503) & perf (PM-0504) backends — BLOCKED, need your environment.**
  The capability layer is done (metrics registered, `capabilities` reports
  availability, absent backends render unavailable — never zero). The *real*
  collectors need a BPF toolchain + CO-RE, elevated privileges (CAP_BPF /
  CAP_PERFMON or a relaxed `perf_event_paranoid`), and a suitable kernel — none
  available or testable in this rootless environment. I stopped rather than ship
  an untestable eBPF loader. The `MetricCollector` port is ready; point me at a
  suitable host/CI (or say "stub is fine") and I'll implement + verify them.

_Last updated by the autonomous build session._
