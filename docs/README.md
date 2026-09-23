# docs/

User- and architecture-facing documentation for `procfit`. These files are part of
"done" (see [`../DEVELOPMENT.md`](../DEVELOPMENT.md) §6) and must stay in lockstep
with the code.

Planned documents (created by the tickets that introduce the feature):

| File | Introduced by | Contents |
|---|---|---|
| `config.yaml` | PM-0002 / PM-0005 | Reference config with every key set to its built-in default (copy to `~/.config/procfit/config.yaml`). Validate with `procfit config check`. |
| `json-schema.md` | PM-0004 / PM-0107 | Versioned JSON/NDJSON output schema (public interface). |
| `configuration.md` | PM-0002 / PM-0005 | Config formats, discovery, merge, presets; examples generated from the canonical model. |
| `cli.md` | PM-0108 / PM-0205 | Command and flag reference with examples. |
| `metrics.md` | PM-0003 / collectors | Metric registry: ids, units, cost, availability. |
| `capabilities.md` | PM-0102 / PM-0109 | Capability model and remediation guidance. |
| `architecture.md` | ongoing | Deep dive into the two-plane design and package boundaries. |

The authoritative product spec remains
[`../RFC-procfit-linux-process-observer-controller.md`](../RFC-procfit-linux-process-observer-controller.md).
