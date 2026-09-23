---
id: PM-0524
title: TUI filter autocomplete (substring vs column-query, operator suggestions)
state: DONE
phase: 3
depends: ["PM-0302"]
owner:
rfc: ["§19"]
---

## Summary

Make the TUI filter line smart about intent and offer autocomplete. A single word
(no spaces) is a case-insensitive **target substring**; adding a space commits to
a **column filter** (`column op value`), with type-aware operator suggestions.

## Scope

- `bareSearch`: one word (no spaces/operators) → `target ~= "(?i)<escaped>"`
  (case-insensitive, metacharacters escaped); anything with a space/operator is
  passed through as an expression.
- Context detection (`filterState`): first word of the buffer = substring position
  (column hints, Enter applies); first word after `&&`/`||` = column (Enter
  autofills); second token = operator (Enter autofills); third+ = value (no
  suggestions, Enter applies).
- Suggestions: displayed filterable columns (prefix, case-insensitive) and, per
  the field's type, operators (`ports`-injected `FilterOperators`: numeric
  comparisons vs string `==`/`~=`/`contains`/`in`).
- Keys: `Enter` autofills in autofilling contexts else applies; `Tab` autofills
  the first suggestion in ANY suggesting context (incl. the substring position).
- Inline suggestion hint in the filter status line, the Enter-autofill target
  bracketed.

## Out of scope

- Value autocompletion (3rd token) and multi-select of suggestions (only the first
  is offered).
- Wiring config presets (`--preset`) — separate.

## Acceptance criteria

- [x] Lone word → case-insensitive target substring; Enter applies.
- [x] After a space, the first word is a column and operators are suggested by type.
- [x] Enter autofills operators / post-`&&` columns; Tab autofills anywhere.
- [x] Third+ token has no suggestions; Enter applies.

## Tests

- `TestBareSearch_WordVsQuery`, `TestFilterAutocomplete` (all four contexts +
  autofill), `TestSuggestHint_MarksAutofillTarget`; existing filter tests updated
  for the case-insensitive substring form.

## Notes

Operator type classification lives in the app (`assembly.filterOperators` /
`isNumericField`) and is injected via `tui.Deps.FilterOperators`, keeping the TUI
package free of the metric/registry knowledge.
