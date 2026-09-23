package metrics

import "github.com/netikras/procfit/internal/model"

// builtinGPU are cost-level-1 per-process GPU metrics from the vendor-neutral DRM
// fdinfo ABI (RFC §13). Utilization is a windowed rate; memory is a gauge. Both
// stay absent for a process that holds no DRM client (never fabricated as zero),
// and cover Intel/AMD/ARM DRM drivers — proprietary NVIDIA is simply invisible.
var builtinGPU = []Descriptor{
	{ID: "gpu", Unit: UnitPercent, Scope: ScopeProcess, Kind: KindRate,
		Cost: Cost1FD, Collector: "gpu", Aggregation: AggSumOncePerProc, Rate: true,
		Description: "per-process GPU engine utilization % (summed across engines, DRM fdinfo)"},
	{ID: "gpu-mem", Unit: UnitBytes, Scope: ScopeProcess, Kind: KindGauge,
		Cost: Cost1FD, Collector: "gpu", Aggregation: AggSumOncePerProc,
		Description: "per-process GPU memory, resident/used (DRM fdinfo)"},
	// Per-engine utilization (PM-0511): driver engine names mapped onto standard
	// classes; cycles-based drivers (drm-cycles/drm-maxfreq) are supported too.
	gpuEngine("gpu-render", "per-process GPU render/3D engine utilization % (DRM fdinfo)"),
	gpuEngine("gpu-compute", "per-process GPU compute engine utilization % (DRM fdinfo)"),
	gpuEngine("gpu-copy", "per-process GPU copy/DMA engine utilization % (DRM fdinfo)"),
	gpuEngine("gpu-video", "per-process GPU video decode/encode engine utilization % (DRM fdinfo)"),
	gpuEngine("gpu-video-enhance", "per-process GPU video-enhance engine utilization % (DRM fdinfo)"),
}

func gpuEngine(id, desc string) Descriptor {
	return Descriptor{
		ID: model.MetricID(id), Unit: UnitPercent, Scope: ScopeProcess, Kind: KindRate,
		Cost: Cost1FD, Collector: "gpu", Aggregation: AggSumOncePerProc, Rate: true, Description: desc,
	}
}
