---
id: PM-0001
title: Go module, CLI skeleton, version/build info
state: TODO
phase: 0
depends: []
owner:
rfc: ["§8", "§24", "§27.Phase0", "§31"]
---

## Summary

Bootstrap the repository as a runnable Go program: module, command dispatch
skeleton, and a `pm version` that reports build metadata. This establishes the
package layout every later ticket plugs into.

## Scope

- `go.mod` (module path, Go toolchain version pinned).
- `cmd/pm/main.go` entry point delegating to `internal/app`.
- `internal/app` command router covering every command name in RFC §8
  (subcommands may be stubs returning "not implemented" with the correct exit
  code semantics from §8.7).
- Global flag surface parsed into a typed struct (RFC §8.1); unknown flags error
  with exit code 2.
- `pm version` prints version, commit, build date via `-ldflags`.
- Bare `pm` on a non-TTY stdout fails helpfully (RFC §7.1) instead of emitting
  terminal sequences.
- Skeleton package directories from RFC §24 created with doc.go stubs.

## Out of scope

- Real behaviour of any subcommand beyond `version`.
- Config loading (PM-0002), registries (PM-0003).

## Acceptance criteria

- [ ] `go build ./...` and `go vet ./...` pass.
- [ ] `pm version` prints injected build info.
- [ ] Unknown command/flag returns exit code 2 with a usage message on stderr.
- [ ] Bare `pm` with redirected stdout exits non-zero with a clear message.
- [ ] Package layout matches RFC §24; no `util` package.

## Tests required

- unit: flag parsing (valid, unknown, exit codes), TTY-detection branch via an
  injected isTerminal seam, version output.
- golden: `pm --help` and `pm version` output (timestamps/commit normalized).

## Notes

Keep the binary name a single constant so it is trivial to rename (RFC §13
warning, §30.1). Command routing must be table-driven so subcommands register
themselves — supports Open/Closed as commands are added per phase.
