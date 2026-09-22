#!/usr/bin/env sh
# Emit procfit's build version as SemVer with a build-metadata identifier:
#
#   <core>+<yyyyMMdd-HHmmss>-<short-sha>[.dirty]
#
# e.g. 1.2.3+20260922-154210-d60fa93  or  0.1.0+20260922-154210-d60fa93.dirty
#
# <core> is the nearest git tag (leading "v" stripped), or 0.1.0 when untagged.
# The build metadata (after '+') is ignored by SemVer precedence but records
# exactly when and from which commit a binary was built. Env overrides (used by
# CI so the tag/sha/time are authoritative): BASE_VERSION, GIT_SHA, BUILD_TIME.
set -eu

base="${BASE_VERSION:-}"
if [ -z "$base" ]; then
	base="$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)"
fi
[ -n "$base" ] || base="0.1.0"

sha="${GIT_SHA:-}"
[ -n "$sha" ] || sha="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"

ts="${BUILD_TIME:-}"
[ -n "$ts" ] || ts="$(date -u +%Y%m%d-%H%M%S)"

dirty=""
if [ -n "$(git status --porcelain 2>/dev/null || true)" ]; then
	dirty=".dirty"
fi

printf '%s+%s-%s%s\n' "$base" "$ts" "$sha" "$dirty"
