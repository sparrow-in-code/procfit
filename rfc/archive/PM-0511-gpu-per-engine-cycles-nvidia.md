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

## Status: DONE (2026-09-23)

Per-engine + cycles fallback shipped in `internal/procfs/gpu.go`. Driver engine
names are mapped onto canonical classes (`canonicalEngine`: render/gfx/3d →
gpu-render, compute → gpu-compute, copy/dma/blitter → gpu-copy, video/dec/enc/bsd/
vcn/jpeg → gpu-video, video-enhance/vebox → gpu-video-enhance) rather than a
dynamic registry — the DRM fdinfo engine classes are standard enough that a
curated static set covers i915/xe/amdgpu/ARM. `drmClient.engineNs` tracks per-class
busy ns (deduped per client), and `applyWindow` emits `gpu-render/compute/copy/
video/video-enhance` as windowed % (an engine the process holds but did not use is
0, not absent; unused-by-driver names stay in summed `gpu`). Cycles-based drivers
(`drm-cycles-<n>` + `drm-maxfreq-<n>`) are converted to ns when `drm-engine-<n>`
(ns) is absent (`applyCyclesFallback`; ns wins to avoid double count). Descriptors
in `builtin_gpu.go`. **NVML deferred** to PM-0522 (build-tagged cgo adapter for the
proprietary NVIDIA driver — needs the NVIDIA library + hardware, untestable in
this sandbox; nouveau already works via DRM fdinfo).

## Acceptance criteria

- [x] Optional per-engine columns render summed-per-engine utilization; absent
      engines degrade to unavailable/0 appropriately, never a misleading zero.
- [x] Cycles-based drivers yield a utilization % via cycles/maxfreq/wall.
- [ ] NVIDIA clients appear via the optional NVML adapter → **deferred to PM-0522**.

## Tests required

- unit: per-engine fixture parse + column projection; cycles→% math; NVML adapter
  behind an interface with a fake.

## Notes

Deferred from PM-0508. The summed `gpu` metric and the `drm-*` parser already
exist; per-engine mostly needs a dynamic-descriptor mechanism, and NVML is an
additive, isolated collector.
