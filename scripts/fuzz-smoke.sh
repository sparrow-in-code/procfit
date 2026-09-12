#!/usr/bin/env bash
# Short fuzz smoke run of every registered fuzz target (RFC §26.2, PM-9001).
# Each target runs briefly; CI uses this to catch crashers early. Deeper fuzzing
# runs on demand with a longer -fuzztime.
set -euo pipefail

FUZZTIME="${FUZZTIME:-10s}"

# Map of package -> fuzz function. Add new targets here.
declare -a TARGETS=(
  "./internal/procfs FuzzParseStat"
  "./internal/procfs FuzzParseIO"
  "./internal/expr FuzzCompile"
)

fail=0
for entry in "${TARGETS[@]}"; do
  pkg="${entry%% *}"
  fn="${entry##* }"
  echo ">>> fuzzing $fn in $pkg for $FUZZTIME"
  if ! go test -run '^$' -fuzz "^${fn}$" -fuzztime "$FUZZTIME" "$pkg"; then
    echo "!!! fuzz target $fn failed"
    fail=1
  fi
done

exit "$fail"
