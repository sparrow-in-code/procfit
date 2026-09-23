---
id: PM-0522
title: NVIDIA GPU via optional build-tagged NVML adapter
state: TODO
phase: 5
depends: ["PM-0511"]
owner:
rfc: ["§13", "§24"]
---

## Summary

The proprietary NVIDIA driver does not emit standard DRM fdinfo, so NVIDIA GPU
clients are invisible to the vendor-neutral collector (PM-0508/0511). Add an
optional, build-tagged NVML-backed collector behind the same `gpu`/`gpu-mem`
(and, where available, per-engine) metrics, so the default build stays free of
any NVML/cgo dependency and the core remains vendor-neutral. nouveau already
works through the DRM path.

## Scope

- A `//go:build nvml`-tagged collector using NVML (per-process utilization +
  memory via `nvmlDeviceGetComputeRunningProcesses` / accounting APIs).
- Register it only in the tagged build; the default build keeps the DRM fdinfo
  collector and no NVML symbols.
- Put NVML behind a small interface so the decode is unit-testable with a fake;
  degrade to unavailable (never zero) when the library/driver is absent.

## Out of scope

- Bundling the NVIDIA library; it is a runtime/link dependency of the tagged build.
- Per-process watts modelling.

## Acceptance criteria

- [ ] Tagged build surfaces NVIDIA per-process gpu/gpu-mem; default build has no
      NVML dependency and is unchanged.
- [ ] NVML access behind an interface with a fake driving the tests.
- [ ] Absent driver/library → unavailable with a clear reason.

## Tests required

- unit: the NVML adapter behind its interface with a fake; capability gating.

## Notes

Split from PM-0511 (per-engine + cycles-based DRM shipped there). NVML needs cgo
and the NVIDIA library plus real hardware, so it is isolated behind a build tag
and validated on an NVIDIA host.
