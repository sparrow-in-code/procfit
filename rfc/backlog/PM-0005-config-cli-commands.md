---
id: PM-0005
title: config check / convert / dump --effective commands
state: TODO
phase: 0
depends: ["PM-0002", "PM-0003"]
owner:
rfc: ["§9.7", "§27.Phase0"]
---

## Summary

Wire the config pipeline to user-facing commands so configuration is verifiable
and portable across formats. This is the Phase 0 exit criterion.

## Scope

- `procfit config check [file]`: strict validation, exit code 2 on failure.
- `procfit config convert <file> --to toml|yaml|json`: lossless conversion via the
  canonical model (not text munging).
- `procfit config dump --effective [--explain]`: fully resolved config after
  defaults + config + preset inheritance + CLI overrides; `--explain` annotates
  each value's source (§9.7).

## Out of scope

- Applying config to a live engine (Phase 1+).

## Acceptance criteria

- [ ] `config check` reports precise errors for every strictness rule (§9.4).
- [ ] `convert` round-trips: TOML→YAML→JSON→TOML yields identical canonical
      config.
- [ ] `dump --effective` reflects preset + CLI override precedence exactly.
- [ ] `--explain` names the source (default/config/preset/flag) per value.

## Tests required

- unit: each command's exit codes and error surfacing.
- golden: effective-config dumps in all three encodings (§26.4); converted
  outputs.

## Notes

**Phase 0 exit criterion (RFC §27):** config equivalence and strictness tests
pass; `check`, `convert`, and `dump --effective` work.
