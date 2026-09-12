---
id: PM-0108
title: pm ps (one-shot) and pm stat (streaming) commands
state: TODO
phase: 1
depends: ["PM-0107"]
owner:
rfc: ["§7.2", "§7.3", "§8.1", "§8.2"]
---

## Summary

The two non-interactive observation commands, wiring global query flags →
QuerySpec → engine → renderer, including two-sample warm-up and append-only
streaming.

## Scope

- `pm ps`: one-shot table. Takes two samples when any requested metric is a rate
  (default warm-up = process collector interval); `--instant` skips the second
  sample and marks rates unavailable (§7.2).
- `pm stat [interval]`: append-only repeated samples (§7.3) — timestamp per row;
  periodic header reprint; never rewrite prior lines; flush per batch; `--count
  N` then exit; `--no-warmup`; `Ctrl-C` exits 130.
- Full global query flag mapping (§8.1) and metric override semantics (§8.2:
  `--metrics`, `--metric +/-`, left-to-right).
- Stable machine output via `--format csv|json|ndjson` (§7.3).

## Out of scope

- Runtime/managed/control commands (Phase 2); TUI (Phase 3); daemon (Phase 4).

## Acceptance criteria

- [ ] `pm ps --group-by comm --leaf none --sort cpu:desc` — correct aggregate CPU
      (MVP §28.1).
- [ ] `pm ps --group-by none --leaf process` — flat list (§28.2).
- [ ] `pm stat 1s --count 3` — three timestamped append-only batches (§28.3).
- [ ] `--instant`/`--no-warmup` render rates unavailable, not zero.
- [ ] `Ctrl-C` during `stat` exits with code 130.
- [ ] Metric profile + `+/-` overrides resolve predictably (§28.6).

## Tests required

- unit: flag→QuerySpec mapping; warm-up/instant behaviour; count/exit-code paths
  using fake clock + fake procfs.
- golden: `ps`/`stat` output across formats (normalized timestamps/PIDs).

## Notes

**Phase 1 exit criterion (RFC §27):** usable as a grouped `ps`/`pidstat` meeting
the light-profile performance budget.
