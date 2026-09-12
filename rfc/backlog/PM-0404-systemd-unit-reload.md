---
id: PM-0404
title: User systemd unit generation/install + safe daemon reload
state: TODO
phase: 4
depends: ["PM-0402"]
owner:
rfc: ["§8", "§17.2", "§17.4", "§20.4"]
---

## Summary

Optional persistence helpers and safe live config reload for the daemon.

## Scope

- `pm service install` / `service uninstall` (§8): write/remove a **user**
  systemd unit. The TUI/CLI may show the enable command but must never silently
  install/enable persistence (§17.2).
- `pm daemon reload` / SIGHUP (§17.4): parse+normalize+validate before swapping
  config; a bad new config leaves the prior active and reports the error.
- Removed policy rules become unmanaged but are NOT implicitly restored (§17.4).
- Config trust in daemon mode (§20.4): refuse config writable by another user
  unless an explicit unsafe flag is given.

## Out of scope

- Restore-on-removal (future rule, §17.4).

## Acceptance criteria

- [ ] Generated unit is user-scoped; install/enable is never automatic/silent.
- [ ] Reload with invalid config keeps the previous config active + reports error.
- [ ] Removed rules unmanage without implicit restore.
- [ ] Foreign-writable config refused in daemon/policy mode (§20.4).

## Tests required

- unit: unit-file generation; reload success/failure (prior config retained);
  rule-removal ⇒ unmanage; config-trust ownership check.

## Notes

**Phase 4 exit criterion (§27):** policy matches new processes after daemon
start/reboot and two clients share one collector schedule (latter in PM-0402).
