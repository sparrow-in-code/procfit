---
id: PM-0508
title: Per-process GPU usage via DRM fdinfo
state: DONE
phase: 5
depends: ["PM-0501"]
owner:
rfc: ["§13", "§24"]
---

## Summary

Add per-process GPU utilization — a significant, currently-invisible laptop drain
source. Modern kernels expose per-client GPU engine busy time and memory in
`/proc/PID/fdinfo/<drm-fd>`, a **vendor-neutral kernel ABI**, so one collector
covers Intel, AMD, and ARM GPUs.

## Scope

- Scan a process's `/proc/PID/fd` for DRM file descriptors, parse the matching
  `/proc/PID/fdinfo/<fd>` `drm-*` keys:
  - `drm-engine-<name>` (ns busy per engine: render/copy/video/compute/…) →
    per-engine utilization % over the interval (delta of ns / wall ns).
  - `drm-total-<region>` / `drm-memory-<region>` → GPU memory gauges.
- Metrics: `gpu` (busiest-engine or summed utilization %), optional per-engine
  columns, `gpu-mem`. Aggregate per process (sum across its DRM fds/engines).
- Capability-gated and degrades to unavailable when no DRM fdinfo is present.
- Reuse the existing fd-scanning collector (PM-0501) machinery.

## Out of scope

- NVIDIA **proprietary** driver, which does not expose the standard drm fdinfo —
  leave a hook for an optional NVML adapter later (nouveau does expose fdinfo).
- GPU power in watts (comes from PM-0506 RAPL psys / hwmon where available).

## Acceptance criteria

- [x] Per-process GPU engine utilization + memory parsed from DRM fdinfo, delta
      over the interval; multiple engines/fds summed per process.
- [x] Vendor-neutral: no driver name hardcoded; works for i915/xe (Intel),
      amdgpu (AMD), and ARM DRM drivers (panfrost/panthor/lima/v3d) that emit
      fdinfo — engine names are read, not assumed.
- [x] No DRM fdinfo → unavailable, never zero. Own-process fds need no root;
      all-process needs privilege.

## Tests required

- unit: fdinfo parser over fixture `drm-engine-*`/`drm-memory-*` payloads;
  multi-fd/multi-engine summation; delta math; absent → unavailable.

## Notes

DRM fdinfo is the portable path across Intel/AMD/ARM. NVIDIA proprietary is the
one gap, isolated behind an optional adapter so the core stays vendor-neutral.

## Status: DONE (2026-09-22)

Shipped `GPUCollector` (`internal/procfs/gpu.go`): finds `/dev/dri/*` fds, parses
their `/proc/PID/fdinfo` `drm-*` keys, and derives `gpu` (engine utilization %,
summed across engines) + `gpu-mem` (resident→memory→total fallback). Metrics are
registered (`builtin_gpu.go`, new `UnitPercent`), added to the `power` and
`battery` profiles, and the collector is wired into the assembly (zero cost unless
requested). Utilization uses a self-contained 200 ms window (like WakeupsCollector,
since `enrich` sees one sample), so a one-shot `ps` still yields a value. Dup'd fds
are deduped by `drm-pdev`/`drm-client-id`; `drm-engine-capacity-*` is excluded from
busy time. Verified live: `procfit metrics` lists both; `ps --metrics power` renders
`GPU`/`GPU-MEM` and correctly shows `?` (permission) / `-` (no client) on this
GPU-less host. Unit tests cover the parser, window delta, dedup, memory fallback,
no-DRM skip, and read-error surfacing. `make check` green (coverage 80.7%).

**Deferred (follow-up ticket worthy):**
- Optional **per-engine columns** (`gpu-render`/`gpu-video`/…) — engine names are
  dynamic, so they need runtime-derived descriptors; only summed `gpu` ships now.
- **Cycles-based drivers** (`drm-cycles-*`/`drm-maxfreq-*`, e.g. some ARM/v3d) —
  only `drm-engine-*` (ns) utilization is parsed today.
- **NVIDIA proprietary** via an optional NVML adapter (nouveau already works).
