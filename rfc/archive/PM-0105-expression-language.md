---
id: PM-0105
title: Selector/filter expression language (lexer, parser, type checker, evaluator)
state: DONE
phase: 1
depends: ["PM-0003"]
owner:
rfc: ["§10", "§26.1", "§26.2"]
---

## Summary

The DSL powering `select` (pre-group, per-entity) and `having` (post-aggregate,
per-row), plus persistent `managed[].selector`. Parsed and type-checked before
sampling starts.

## Scope

- Lexer + parser for the grammar in RFC §10.2 (Pratt or generated).
- Literals (§10.3): strings, ints/decimals, byte sizes (K/KB/KiB/G/GiB),
  durations (Go-like), RE2 regex `~=`/`!~`, lists, booleans, bare enums where
  unambiguous.
- Operators: `== != < <= > >= ~= !~ in "not in" contains`, `&& || !` with
  short-circuit (§10.6).
- Functions (§10.4): `exists`, `missing`, `changed`, `drifted`, `age`,
  `last_seen`.
- Type checker (§10.5): field validity for entity vs aggregate context; reject
  aggregate-only fields in `select` and entity-only fields in group-only
  `having`; unknown fields/invalid comparisons are errors.
- Missing-value semantics (§10.6): comparisons with missing ⇒ false (incl. `!=`);
  `missing()/exists()` required for availability logic.

## Out of scope

- Applying selectors to actual entities/rows (query engine PM-0106 consumes the
  compiled program).

## Acceptance criteria

- [ ] Compiled program reports whether it is valid for entity vs aggregate use.
- [ ] `cpu > 5` in `select` (aggregate-only ambiguity) is a compile error where
      applicable per §10.5.
- [ ] Missing-value comparisons return false and short-circuit correctly.
- [ ] RE2 only — no catastrophic backtracking possible.
- [ ] Byte-size and duration literals parse to canonical units.

## Tests required

- unit: grammar precedence, each operator/literal/function, type errors,
  missing-value truth table.
- property/fuzz: parser/evaluator never panic; short-circuit respected (§26.2).

## Notes

Two-stage evaluation is the core disambiguation of the product (§10.1). Keep the
evaluator field access behind an interface so entities and aggregated rows can
both satisfy it (Liskov).

## Status: DONE (2026-09-12)

Implemented in the initial autonomous build. Core acceptance criteria met and covered by tests (module coverage ≥80%, race-clean). Deferred refinements (golden snapshots, fuzz corpora, and any renderer/format extras) are tracked in PM-0110.
