---
id: PM-0511
title: GPU extras — per-engine columns, cycles-based drivers, NVIDIA adapter
state: TODO
phase: 5
depends: ["PM-0508"]
owner:
rfc: ["§13", "§24"]
---

## Summary

Extend the DRM fdinfo GPU collector (PM-0508, which ships summed `gpu` +
`gpu-mem`) with the pieces deliberately deferred to keep that change tight.

## Scope

- **Per-engine utilization columns** (`gpu-render`, `gpu-video`, `gpu-copy`,
  `gpu-compute`, …). Engine names are driver-defined and only known at runtime,
  so this needs runtime-derived descriptors/columns (or a dynamic metric family)
  rather than the static registry — design that seam.
- **Cycles-based drivers:** derive utilization from `drm-cycles-<engine>` +
  `drm-maxfreq-<engine>` when `drm-engine-<engine>` (ns) is absent (some ARM/v3d
  and other drivers report cycles, not nanoseconds).
- **NVIDIA proprietary via optional NVML adapter:** the closed driver does not
  emit standard DRM fdinfo. Add an optional, build-tagged NVML-backed collector
  behind the same `gpu`/`gpu-mem` metrics so the core stays vendor-neutral
  (nouveau already works through PM-0508).

## Out of scope

- GPU power in watts (PM-0506 RAPL psys / hwmon).

## Acceptance criteria

- [ ] Optional per-engine columns render summed-per-engine utilization; absent
      engines degrade to unavailable, never zero.
- [ ] Cycles-based drivers yield a utilization % via cycles/maxfreq/wall.
- [ ] NVIDIA clients appear via the optional NVML adapter when present; the core
      build has no NVML dependency.

## Tests required

- unit: per-engine fixture parse + column projection; cycles→% math; NVML adapter
  behind an interface with a fake.

## Notes

Deferred from PM-0508. The summed `gpu` metric and the `drm-*` parser already
exist; per-engine mostly needs a dynamic-descriptor mechanism, and NVML is an
additive, isolated collector.
