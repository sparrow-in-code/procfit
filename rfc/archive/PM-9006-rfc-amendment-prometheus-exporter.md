---
id: PM-9006
title: "RFC amendment: Prometheus /metrics exporter (scrapable, query-selectable)"
state: DONE
phase: 5
depends: ["PM-0402", "PM-0107"]
owner:
rfc: ["§18"]
---

## Status: DONE (2026-09-23)

Amendment implemented (fold into RFC §18 on the next master-RFC revision). New
renderer `internal/render/prometheus` emits text exposition format: process/group
metric families labelled `target` (+ `pid` for process rows), host-scoped metrics
as unlabelled families, names `procfit_<id>` (non-alnum → `_`), HELP from the
descriptor; a family with no available samples is omitted entirely (no fabricated
zeros). Hosted by a new `procfit export` command (`internal/app/export.go`):
one-shot to stdout, or `--listen ADDR` serving `GET /metrics` with per-scrape URL
overrides (`profile`/`group_by`/`leaf`/`select`/`having`). It reuses the full
assembly (all collectors) and the warm-up sample, so rate metrics stay meaningful
per scrape (the daemon's light-only RunQuery can't). Response is buffered so a
client disconnect can't double-write. Validated live (one-shot + HTTP scrape with
`?profile=` override). **Deferred:** the daemon-hosted continuous endpoint (avoids
per-scrape warm-up) and endpoint auth beyond bind-address — noted in the README.

## Summary

Amend the RFC to expose metrics in Prometheus text exposition format for scraping
and export. The engine already produces the values and renderers are pure
presentation, so this is a new renderer plus an HTTP endpoint — naturally hosted
by the long-running daemon (shared collection). The scrape is query-selectable so
different jobs can pull different views.

## Proposed amendment

- **New renderer** `internal/render/prometheus`: emit metric families from a
  `query.Result` in text exposition format, with labels drawn from the row
  identity (pid, comm, user, host-scoped metrics as their own families).
- **HTTP endpoint** on `procfit daemon` (opt-in `--metrics-addr`, default off):
  `GET /metrics?profile=network&group_by=comm&select=...` maps query params to
  `queryspec.Flags` and renders the current sample.
- **Cardinality guard:** default to an aggregated view (e.g. `group_by comm` or
  `user`) with comm/user/pid labels; raw per-pid series are explicit opt-in and
  documented as high-cardinality.
- Versioned/stable metric names and label sets (a public interface, like the JSON
  schema); one-shot `procfit export --prometheus` MAY also be offered for cron.

## Out of scope

- Pushgateway / remote-write; auth on the endpoint beyond bind-address control
  (front with a reverse proxy if exposed).
- Histograms/summaries (gauges/counters first).

## Acceptance criteria

- [x] `/metrics` returns valid exposition format; `promtool check metrics` clean.
- [x] Query params select profile/group-by/columns/select; cardinality-guarded
      default.
- [x] Unavailable metrics are omitted (or `NaN`), never fabricated zeros; names
      are stable/versioned.

## Tests required

- unit: prometheus renderer golden over a fixture Result (families, labels,
  HELP/TYPE); query-param → Flags mapping; cardinality default.

## Notes

Host-scoped metrics (power/C-state/PSI) become their own no-label (or host-label)
families. New output surface not in RFC §18 → present as an amendment first.
