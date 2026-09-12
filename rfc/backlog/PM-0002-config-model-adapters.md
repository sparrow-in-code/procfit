---
id: PM-0002
title: Canonical config model + strict TOML/YAML/JSON adapters + merge/validate
state: TODO
phase: 0
depends: ["PM-0001"]
owner:
rfc: ["§9", "§26.1", "§26.2"]
---

## Summary

One canonical configuration DTO fed by three strict decoder adapters (TOML,
YAML, JSON) with identical semantics, followed by normalization, merge, and
semantic validation. This is the backbone that makes config "code-like intent"
safe and format-agnostic.

## Scope

- Canonical `Config` model + raw DTO (RFC §9.1 pipeline).
- Strict adapters per format: reject unknown fields, duplicate keys, type
  mismatches (RFC §9.4). JSON gets its own adapter/policy (RFC §9.1).
- File discovery with precedence and multi-default-file error (RFC §9.2).
- Merge precedence: defaults < config defaults < preset(+ancestors) < CLI
  (RFC §9.3); scalar replace, map merge, array replace; metrics
  profile/enable/disable semantics.
- Preset inheritance with cycle + missing-parent rejection (RFC §9.3/§9.4).
- Semantic validation: enums, durations, impossible combos like
  `group_by=["none","app"]`, dynamic-field control rules (§9.4).

## Out of scope

- `config check/convert/dump` CLI wiring (PM-0005).
- Metric/dimension name existence checks depend on registries (PM-0003) — inject
  a validator interface to avoid an import cycle.

## Acceptance criteria

- [ ] Identical TOML/YAML/JSON inputs resolve to byte-identical canonical config.
- [ ] Unknown key, duplicate key, type mismatch, bad enum, negative/invalid
      duration each produce a validation error in all three formats.
- [ ] Preset inheritance cycles and missing parents are rejected.
- [ ] Merge precedence and array-replace/map-merge behave exactly per §9.3.
- [ ] Absent default config is not an error; multiple defaults is an error.

## Tests required

- unit: per-rule strictness in each format; discovery precedence table.
- property/fuzz: decoder round-trips canonical values; malformed input never
  panics (RFC §26.2).
- golden: `dump --effective` fixtures (shared with PM-0005).

## Notes

Keep decoder adapters behind a `Decoder` interface (one per format) so formats
are open for extension, closed for modification. Validation is a separate stage
from decoding — do not couple syntax errors to semantic errors.
