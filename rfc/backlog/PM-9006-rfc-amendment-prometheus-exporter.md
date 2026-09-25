---
id: PM-9006
title: "RFC amendment: Prometheus /metrics exporter (scrapable, query-selectable)"
state: TODO
phase: 5
depends: ["PM-0402", "PM-0107"]
owner:
rfc: ["§18"]
---

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

- [ ] `/metrics` returns valid exposition format; `promtool check metrics` clean.
- [ ] Query params select profile/group-by/columns/select; cardinality-guarded
      default.
- [ ] Unavailable metrics are omitted (or `NaN`), never fabricated zeros; names
      are stable/versioned.

## Tests required

- unit: prometheus renderer golden over a fixture Result (families, labels,
  HELP/TYPE); query-param → Flags mapping; cardinality default.

## Notes

Host-scoped metrics (power/C-state/PSI) become their own no-label (or host-label)
families. New output surface not in RFC §18 → present as an amendment first.
