---
id: PM-0523
title: TUI system-metrics panel for host-scoped metrics
state: TODO
phase: 5
depends: ["PM-0507"]
owner:
rfc: ["§13", "§19", "§24"]
---

## Summary

Host-scoped metrics (power, C-state residency, PSI) are now collected once and
rendered in a "system section" above the process table for the CLI `ps`/`stat`
table and as a `host` object in JSON. The interactive TUI does not render this
section yet: `query.Result.HostMetrics` is populated but the TUI `Frame()` does
not display it, and host metrics are (deliberately) excluded from the TUI column
picker so they don't show as empty columns. Add a compact system panel to the TUI.

## Scope

- Render `Result.HostMetrics` in the TUI as a small header/panel (the aligned
  `label=value` matrix, reusing the CLI `layoutMatrix` formatting), refreshed on
  the same loop as the table.
- Ensure the host collection runs for the TUI refresh path (thread `HostMetrics`
  through the TUI `RefreshFunc`/`Deps` the same way the CLI does).
- Keep it out of the way when there are no host metrics (no wasted rows).

## Out of scope

- A separate host/cgroup **row** in the tree (host metrics stay a panel, not a
  grouped row); per-cgroup PSI is PM-0521.

## Acceptance criteria

- [ ] `--profile power`/`pressure`/`battery` in the TUI shows the host values in a
      panel, not as empty columns.
- [ ] Panel updates each refresh; no per-process broadcast reintroduced.
- [ ] No host metric appears as a per-process TUI column.

## Tests required

- unit: the TUI Model renders a provided HostMetrics map into the panel (headless
  Model test, like the existing frame tests).

## Notes

Split from the CLI system-section work. The formatting helper (`layoutMatrix`) is
already shared-able; lift it to `internal/render` if the TUI needs it outside
`internal/app`.
