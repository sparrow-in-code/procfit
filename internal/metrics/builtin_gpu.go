package metrics

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
}
