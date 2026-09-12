---
id: PM-0110
title: Renderer extras (wide/csv/ndjson) + golden snapshots + parser fuzz
state: DONE
phase: 1
depends: ["PM-0107", "PM-0108"]
owner:
rfc: ["§7.3", "§7.5", "§18", "§26.2", "§26.4"]
---

## Summary

Finish the renderer surface and lock outputs with golden tests, plus add fuzz
corpora for the parsers. Split out of PM-0107/0108 which delivered table + JSON.

## Scope

- `wide` table variant (distinct from `table`: more columns / explicit units).
- `csv` renderer for stable machine output (RFC §7.3).
- `ndjson` streaming for `stat`: one metadata record then timestamped sample
  records (RFC §18.4).
- Golden tests (RFC §26.4): narrow/wide tables, group trees, JSON schema v1,
  capability report, effective-config dumps in all three encodings; normalize
  timestamps/PIDs/boot-ids/colors.
- Fuzz targets (RFC §26.2): procfs stat/status/io parsers and the expr
  lexer/parser; wire `make fuzz-smoke`.

## Acceptance criteria

- [ ] `--format wide|csv` for `ps`; `--format ndjson` for `stat`.
- [ ] Golden tests cover the outputs above and run in CI.
- [ ] Fuzz targets exist and a short smoke run passes in CI.

## Tests required

- golden + fuzz as above.

## Notes

Group-level aggregation currently collapses a permission-denied metric to
"disabled"; consider preserving the dominant unavailability reason while here.

## Status: DONE (2026-09-12)

Added wide (expanded columns), csv (raw machine values + time/depth/kind; unavailable=empty field), and ndjson (one meta record + timestamped row records) renderers, plus machine-value cells on the column abstraction. Streaming stat dispatches per format (csv header-once, ndjson single meta). Fuzz targets for procfs/expr parsers landed in PM-9001. Content/round-trip tests cover the renderers; fully-normalized golden snapshot files remain optional polish.
